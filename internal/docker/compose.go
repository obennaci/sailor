package docker

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const BackupSuffix = ".sailor-backup"

// ComposeFile represents a parsed docker-compose.yml as a yaml.Node tree.
type ComposeFile struct {
	Path string
	Root *yaml.Node // Document node
}

// ParseCompose reads and parses a docker-compose.yml.
func ParseCompose(path string) (*ComposeFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", path, err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("cannot parse %s: %w", path, err)
	}

	return &ComposeFile{Path: path, Root: &doc}, nil
}

// Save writes the compose file back to disk.
func (c *ComposeFile) Save() error {
	data, err := yaml.Marshal(c.Root)
	if err != nil {
		return err
	}
	return os.WriteFile(c.Path, data, 0644)
}

// Backup creates a backup of the compose file.
func (c *ComposeFile) Backup() error {
	backupPath := c.Path + BackupSuffix
	if _, err := os.Stat(backupPath); err == nil {
		return nil // backup already exists
	}
	data, err := os.ReadFile(c.Path)
	if err != nil {
		return err
	}
	return os.WriteFile(backupPath, data, 0644)
}

// RestoreBackup restores the compose file from backup.
func RestoreBackup(composePath string) error {
	backupPath := composePath + BackupSuffix
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return err
	}
	if err := os.WriteFile(composePath, data, 0644); err != nil {
		return err
	}
	return os.Remove(backupPath)
}

// DetectAppService finds the app service name (usually laravel.test).
func (c *ComposeFile) DetectAppService() string {
	services := c.getServicesNode()
	if services == nil {
		return "laravel.test"
	}

	for i := 0; i < len(services.Content)-1; i += 2 {
		name := services.Content[i].Value
		if name == "laravel.test" {
			return name
		}
	}

	// Fallback: first service
	if len(services.Content) >= 2 {
		return services.Content[0].Value
	}

	return "laravel.test"
}

// DetectInfraServices returns all service names except the app service.
func (c *ComposeFile) DetectInfraServices(appService string) []string {
	services := c.getServicesNode()
	if services == nil {
		return nil
	}

	var infra []string
	for i := 0; i < len(services.Content)-1; i += 2 {
		name := services.Content[i].Value
		if name != appService {
			infra = append(infra, name)
		}
	}
	return infra
}

// HasSharedNetwork checks if the compose file already has the shared network configured.
func (c *ComposeFile) HasSharedNetwork(networkName string) bool {
	networks := c.getTopLevelMapping("networks")
	if networks == nil {
		return false
	}
	for i := 0; i < len(networks.Content)-1; i += 2 {
		if networks.Content[i].Value == "shared" {
			return true
		}
	}
	return false
}

// PatchMainCompose adds the shared network to all services and the top-level networks block.
func (c *ComposeFile) PatchMainCompose(networkName string) error {
	if c.HasSharedNetwork(networkName) {
		return nil // already patched
	}

	services := c.getServicesNode()
	if services == nil {
		return fmt.Errorf("no 'services' found in %s", c.Path)
	}

	// Add "shared" to each service's networks list
	for i := 0; i < len(services.Content)-1; i += 2 {
		serviceBody := services.Content[i+1]
		c.addNetworkToService(serviceBody, "shared")
	}

	// Add shared network definition to top-level networks
	c.addSharedNetworkDefinition(networkName)

	return nil
}

// PatchWorktreeCompose modifies the worktree's compose for app-only mode.
func (c *ComposeFile) PatchWorktreeCompose(appService string, infraServices []string, appPort, vitePort int, networkName string) error {
	services := c.getServicesNode()
	if services == nil {
		return fmt.Errorf("no 'services' found in %s", c.Path)
	}

	for i := 0; i < len(services.Content)-1; i += 2 {
		name := services.Content[i].Value
		serviceBody := services.Content[i+1]

		if name == appService {
			c.patchAppService(serviceBody, appPort, vitePort)
		} else if contains(infraServices, name) {
			c.disableService(serviceBody)
		}
	}

	c.addSharedNetworkDefinition(networkName)

	return nil
}

// getServicesNode returns the mapping node under "services:".
func (c *ComposeFile) getServicesNode() *yaml.Node {
	return c.getTopLevelMapping("services")
}

// getTopLevelMapping finds a top-level key in the YAML document.
func (c *ComposeFile) getTopLevelMapping(key string) *yaml.Node {
	if c.Root == nil || len(c.Root.Content) == 0 {
		return nil
	}
	root := c.Root.Content[0]
	for i := 0; i < len(root.Content)-1; i += 2 {
		if root.Content[i].Value == key {
			return root.Content[i+1]
		}
	}
	return nil
}

// addNetworkToService adds a network name to a service's networks list.
func (c *ComposeFile) addNetworkToService(serviceBody *yaml.Node, network string) {
	// Find existing networks key
	for i := 0; i < len(serviceBody.Content)-1; i += 2 {
		if serviceBody.Content[i].Value == "networks" {
			netNode := serviceBody.Content[i+1]
			// Check if already present
			for _, item := range netNode.Content {
				if item.Value == network {
					return
				}
			}
			netNode.Content = append(netNode.Content, &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: network,
			})
			return
		}
	}

	// No networks key — add one
	serviceBody.Content = append(serviceBody.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "networks"},
		&yaml.Node{
			Kind: yaml.SequenceNode,
			Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Value: "sail"},
				{Kind: yaml.ScalarNode, Value: network},
			},
		},
	)
}

// addSharedNetworkDefinition adds the shared external network to top-level networks.
func (c *ComposeFile) addSharedNetworkDefinition(networkName string) {
	root := c.Root.Content[0]
	var networksNode *yaml.Node

	for i := 0; i < len(root.Content)-1; i += 2 {
		if root.Content[i].Value == "networks" {
			networksNode = root.Content[i+1]
			break
		}
	}

	if networksNode == nil {
		// Create networks top-level key
		networksNode = &yaml.Node{Kind: yaml.MappingNode}
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "networks"},
			networksNode,
		)
	}

	// Check if shared already exists
	for i := 0; i < len(networksNode.Content)-1; i += 2 {
		if networksNode.Content[i].Value == "shared" {
			return
		}
	}

	// Add shared network
	networksNode.Content = append(networksNode.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "shared"},
		&yaml.Node{
			Kind: yaml.MappingNode,
			Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Value: "external"},
				{Kind: yaml.ScalarNode, Value: "true", Tag: "!!bool"},
				{Kind: yaml.ScalarNode, Value: "name"},
				{Kind: yaml.ScalarNode, Value: networkName},
			},
		},
	)
}

// patchAppService modifies the app service for worktree mode.
func (c *ComposeFile) patchAppService(serviceBody *yaml.Node, appPort, vitePort int) {
	for i := 0; i < len(serviceBody.Content)-1; i += 2 {
		key := serviceBody.Content[i].Value

		switch key {
		case "ports":
			// Replace ports
			serviceBody.Content[i+1] = &yaml.Node{
				Kind: yaml.SequenceNode,
				Content: []*yaml.Node{
					{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%d:80", appPort), Style: yaml.SingleQuotedStyle},
					{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%d:5173", vitePort), Style: yaml.SingleQuotedStyle},
				},
			}
		case "networks":
			// Replace networks with only shared
			serviceBody.Content[i+1] = &yaml.Node{
				Kind: yaml.SequenceNode,
				Content: []*yaml.Node{
					{Kind: yaml.ScalarNode, Value: "shared"},
				},
			}
		case "depends_on":
			// Clear depends_on
			serviceBody.Content[i+1] = &yaml.Node{
				Kind: yaml.SequenceNode,
			}
		}
	}
}

// disableService adds profiles: ['disabled'] to a service.
func (c *ComposeFile) disableService(serviceBody *yaml.Node) {
	// Check if profiles already set
	for i := 0; i < len(serviceBody.Content)-1; i += 2 {
		if serviceBody.Content[i].Value == "profiles" {
			serviceBody.Content[i+1] = &yaml.Node{
				Kind: yaml.SequenceNode,
				Content: []*yaml.Node{
					{Kind: yaml.ScalarNode, Value: "disabled"},
				},
			}
			return
		}
	}

	serviceBody.Content = append(serviceBody.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "profiles"},
		&yaml.Node{
			Kind: yaml.SequenceNode,
			Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Value: "disabled"},
			},
		},
	)
}

// SanitizeDBName cleans a string for use as a database name.
func SanitizeDBName(name string) string {
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "-", "_")
	var clean strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			clean.WriteRune(r)
		}
	}
	result := clean.String()
	if len(result) > 64 {
		result = result[:64]
	}
	return result
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
