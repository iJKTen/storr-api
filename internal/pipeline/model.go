package pipeline

// HydrogenStatus represents pipeline status for hydrogen stores.
type HydrogenStatus string

const (
	HydrogenStatusIdle         HydrogenStatus = "idle"
	HydrogenStatusProvisioning HydrogenStatus = "provisioning"
	HydrogenStatusError        HydrogenStatus = "error"
	HydrogenStatusCloning      HydrogenStatus = "cloning"
	HydrogenStatusParsing      HydrogenStatus = "parsing"
	HydrogenStatusGenerating   HydrogenStatus = "generating"
	HydrogenStatusServing      HydrogenStatus = "serving"
)

// HydrogenStatusResponse is the API response for hydrogen pipelines.
type HydrogenStatusResponse struct {
	Status     HydrogenStatus `json:"status"`
	PreviewURL string         `json:"previewUrl,omitempty"`
	Message    string         `json:"message,omitempty"`
}

// ThemeStatus represents pipeline status for theme stores.
type ThemeStatus string

const (
	ThemeStatusIdle               ThemeStatus = "idle"
	ThemeStatusProvisioning       ThemeStatus = "provisioning"
	ThemeStatusError              ThemeStatus = "error"
	ThemeStatusAuthenticating     ThemeStatus = "authenticating"
	ThemeStatusWaitingForAuth     ThemeStatus = "waiting_for_auth"
	ThemeStatusListingThemes      ThemeStatus = "listing_themes"
	ThemeStatusThemeSelection     ThemeStatus = "theme_selection"
	ThemeStatusDownloading        ThemeStatus = "downloading"
	ThemeStatusStartingPreview    ThemeStatus = "starting_preview"
	ThemeStatusWaitingForPassword ThemeStatus = "waiting_for_password"
	ThemeStatusRunning            ThemeStatus = "running"
)

// ThemeStatusResponse is the API response for theme pipelines.
type ThemeStatusResponse struct {
	Status           ThemeStatus `json:"status"`
	PreviewURL       string      `json:"previewUrl,omitempty"`
	AuthURL          string      `json:"authUrl,omitempty"`
	Themes           []ThemeInfo `json:"themes,omitempty"`
	ExistingThemeIDs []string    `json:"existingThemeIds,omitempty"`
}

// ThemeInfo describes a Shopify theme available for selection during the theme pipeline flow.
type ThemeInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// ThemeSelection represents the user's choice of which Shopify theme to use for the pipeline.
type ThemeSelection struct {
	ThemeID     string `json:"themeId"`
	UseExisting bool   `json:"useExisting"`
}

// TaskConfig holds the ECS cluster and task definition identifiers for launching pipeline tasks.
type TaskConfig struct {
	ClusterName    string
	TaskDefinition string
}

// NetworkConfig holds the VPC networking parameters for pipeline ECS tasks and ALB routing.
type NetworkConfig struct {
	SubnetIDs       []string
	SecurityGroupID string
	AlbListenerArn  string
	VpcID           string
}

// Config groups the task and network configuration for the Hydrogen pipeline service.
type Config struct {
	Task    TaskConfig
	Network NetworkConfig
}
