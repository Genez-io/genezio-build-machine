package workflows

import (
	"build-machine/internal"
	"build-machine/service"
	statemanager "build-machine/state_manager"
	"build-machine/utils"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type S3DeploymentArgo struct {
	S3Deployment
	WebhookSecret       string                    `json:"webhookSecret"`
	Token               string                    `json:"token"`
	CodeAlreadyUploaded bool                      `json:"codeAlreadyUploaded"`
	ArgoClient          service.ArgoService       `json:"-"`
	StateManager        statemanager.StateManager `json:"-"`
}

// AssignEnvVarsFromStageID implements Workflow.
// Note: this function will check if any envVars have already been set and merge them with the new ones.
func (d *S3DeploymentArgo) AssignEnvVarsFromStageID(envVars map[string]string) {
	if d.EnvVars == nil {
		d.EnvVars = envVars
		return
	}

	for key, value := range envVars {
		d.EnvVars[key] = value
	}
}

// AssignStateManager implements Workflow.
func (d *S3DeploymentArgo) AssignStateManager(state statemanager.StateManager) {
	d.StateManager = state
}

// Validated implements Workflow.
func (d *S3DeploymentArgo) Validate(args json.RawMessage) error {
	err := json.Unmarshal(args, &d)
	if err != nil {
		return err
	}

	d.CodeAlreadyUploaded = d.S3DownloadURL != ""

	if d.ProjectName == "" {
		return fmt.Errorf("projectName is required")
	}

	if d.Region == "" {
		return fmt.Errorf("region is required")
	}

	if d.Token == "" {
		return fmt.Errorf("token is required")
	}

	if !d.CodeAlreadyUploaded {
		if d.Code == nil {
			return fmt.Errorf("if code has not been uploaded to s3 previously, codemap is required")
		}
	}

	return nil
}

func (d *S3DeploymentArgo) uploadCode() error {
	tmpFolderPath := utils.CreateTempFolder()
	archivePath, err := utils.WriteCodeMapToDirAndZip(d.Code, tmpFolderPath)
	log.Println("Archive path", archivePath)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpFolderPath)

	s3URLUpload, err := utils.UploadContentToS3(archivePath, d.ProjectName, d.Region, d.Stage, d.Token)
	if err != nil {
		return err
	}
	// Parse upload s3 url to extract key
	s3ParsedURL, err := url.Parse(s3URLUpload)
	if err != nil {
		return err
	}

	uploadKey := strings.TrimLeft(s3ParsedURL.Path, "/")
	// Call service
	bucketBaseName := internal.GetConfig().BucketBaseName
	log.Println("Bucket base name", bucketBaseName)
	s3URLDownload, err := utils.DownloadFromS3PresignedURL(d.Region, fmt.Sprintf("%s-%s", bucketBaseName, d.Region), uploadKey)
	if err != nil {
		return err
	}
	d.S3DownloadURL = s3URLDownload
	return nil
}

// Submit implements Workflow.
func (d *S3DeploymentArgo) Submit() (string, error) {
	if !d.CodeAlreadyUploaded {
		err := d.uploadCode()
		if err != nil {
			return "", err
		}
	}
	d.WebhookSecret = uuid.NewString()
	renderedWorkflow := d.RenderArgoTemplate()
	wf_id, err := d.ArgoClient.SubmitWorkflow(renderedWorkflow)
	if err != nil {
		return "", err
	}

	if err := d.StateManager.AttachJobIdToWebhookSecretRef(d.WebhookSecret, wf_id); err != nil {
		return "", err
	}

	if err := d.StateManager.CreateState(wf_id, d.Token, "argo"); err != nil {
		return "", err
	}

	return wf_id, nil
}

func NewS3ArgoDeployment(token string) Workflow {
	argoService := service.NewArgoService()
	return &S3DeploymentArgo{
		Token:      token,
		ArgoClient: *argoService,
	}
}

func (d *S3DeploymentArgo) RenderArgoTemplate() wfv1.Workflow {
	s3FilePerms := int32(0755)

	templateName := "build-s3"
	templateRef := "genezio-build-s3-template"
	generateName := "genezio-build-s3-"

	switch internal.GetConfig().Env {
	case "dev":
		templateName = "build-s3-dev"
		templateRef = "genezio-build-s3-template-dev"
		generateName = "genezio-build-s3-dev-"
	case "local":
		templateName = "build-s3-local"
		templateRef = "genezio-build-s3-template-local"
		generateName = "genezio-build-s3-local-"
	}

	// Encoded params
	serializedParams, err := json.Marshal(d)
	if err != nil {
		log.Println("Error serializing params", err)
	}

	encodedParams := base64.StdEncoding.EncodeToString(serializedParams)
	encodedParamsAS := wfv1.ParseAnyString(encodedParams)
	return wfv1.Workflow{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: generateName,
		},
		Spec: wfv1.WorkflowSpec{
			Entrypoint:         templateName,
			ServiceAccountName: "argo-workflow",
			Templates: []wfv1.Template{
				{
					Name: templateName,
					Steps: []wfv1.ParallelSteps{
						{
							Steps: []wfv1.WorkflowStep{
								{
									Name: "genezio-deploy",
									TemplateRef: &wfv1.TemplateRef{
										Name:     templateRef,
										Template: templateName,
									},
									Arguments: wfv1.Arguments{
										Artifacts: []wfv1.Artifact{
											{
												Name: "codeArchive",
												Path: "/tmp/projectCode.zip",
												Mode: &s3FilePerms,
												ArtifactLocation: wfv1.ArtifactLocation{
													HTTP: &wfv1.HTTPArtifact{
														URL: d.S3DownloadURL,
													},
												},
											},
										},
										Parameters: []wfv1.Parameter{
											{
												Name:  "b64",
												Value: &encodedParamsAS,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
