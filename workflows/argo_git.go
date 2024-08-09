package workflows

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"

	wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"build-machine/internal"
	"build-machine/service"
	statemanager "build-machine/state_manager"
)

type GitDeploymentArgo struct {
	GitDeployment
	WebhookSecret string                    `json:"webhookSecret"`
	Token         string                    `json:"token"`
	ArgoClient    service.ArgoService       `json:"-"`
	StateManager  statemanager.StateManager `json:"-"`
}

// AssignEnvVarsFromStageID implements Workflow.
// Note: this function will check if any envVars have already been set and merge them with the new ones.
func (d *GitDeploymentArgo) AssignEnvVarsFromStageID(envVars map[string]string) {
	if d.EnvVars == nil {
		d.EnvVars = envVars
		return
	}

	for key, value := range envVars {
		d.EnvVars[key] = value
	}
}

// AssignStateManager implements Workflow.
func (d *GitDeploymentArgo) AssignStateManager(state statemanager.StateManager) {
	d.StateManager = state
}

// Submit implements Workflow.
func (d *GitDeploymentArgo) Submit() (string, error) {
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

// Validated implements Workflow.
func (d *GitDeploymentArgo) Validate(args json.RawMessage) error {
	err := json.Unmarshal(args, &d)
	if err != nil {
		return err
	}

	if d.Repository == "" {
		return fmt.Errorf("repository is required")
	}

	return nil
}

func NewGitArgoWorkflow(token string) Workflow {
	argoService := service.NewArgoService()

	return &GitDeploymentArgo{
		Token:      token,
		ArgoClient: *argoService,
	}
}

func (d *GitDeploymentArgo) RenderArgoTemplate() wfv1.Workflow {
	// Encoded params
	serializedParams, err := json.Marshal(d)
	if err != nil {
		log.Println("Error serializing params", err)
	}

	encodedParams := base64.StdEncoding.EncodeToString(serializedParams)
	encodedParamsAS := wfv1.ParseAnyString(encodedParams)

	templateName := "build-git"
	templateRef := "genezio-build-git-template"
	generateName := "genezio-build-git-"

	switch internal.GetConfig().Env {
	case "dev":
		templateName = "build-git-dev"
		templateRef = "genezio-build-git-template-dev"
		generateName = "genezio-build-git-dev-"
	case "local":
		templateName = "build-git-local"
		templateRef = "genezio-build-git-template-local"
		generateName = "genezio-build-git-local-"

	}
	return wfv1.Workflow{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: generateName,
		},
		Spec: wfv1.WorkflowSpec{
			Entrypoint:         templateName,
			ServiceAccountName: "argo-workflow",
			PodSpecPatch:       `{"containers":[{"name":"main","env":[{"name":"GENEZIO_WH_SECRET","value":"` + d.WebhookSecret + `"}]}]}`,
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
