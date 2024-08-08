package dtos

// Specific input definitions for each workflow type
type GitDeployment struct {
	Repository   string   `json:"githubRepository"`
	ProjectName  string   `json:"projectName"`
	Region       string   `json:"region"`
	Stage        string   `json:"stage"`
	BasePath     *string  `json:"basePath,omitempty"`
	Stack        []string `json:"stack,omitempty"`
	IsNewProject bool     `json:"isNewProject"`
}

type File struct {
    Content string `json:"content"`
    IsBase64Encoded bool `json:"isBase64Encoded"`
}

type S3Deployment struct {
	S3DownloadURL string            `json:"s3DownloadURL,omitempty"`
	ProjectName   string            `json:"projectName"`
	Stage         string            `json:"stage"`
	Region        string            `json:"region"`
	BasePath      *string           `json:"basePath,omitempty"`
	Code          map[string]File `json:"code"`
}
