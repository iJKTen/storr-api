package pipeline

// ThemeConfig holds the ECS cluster, task definition, container name, and network
// parameters for the theme pipeline service.
type ThemeConfig struct {
	ClusterName    string
	TaskDefinition string
	ContainerName  string
	Network        NetworkConfig
}
