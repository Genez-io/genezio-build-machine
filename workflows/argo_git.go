package workflows

import (
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
	WebhookSecret string
	Token         string
	ArgoClient    service.ArgoService
	StateManager  statemanager.StateManager
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
	//convert args to string and print it
	log.Printf("args = %v", string(args))
	log.Printf("Args: %v %v %v", d.ProjectName, d.Repository, d.Region)
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
	tokenAS := wfv1.ParseAnyString(d.Token)
	repoAS := wfv1.ParseAnyString(d.Repository)
	regionAS := wfv1.ParseAnyString(d.Region)
	projectnameAS := wfv1.ParseAnyString(d.ProjectName)
	basePathAS := wfv1.ParseAnyString("")
	stackAS := wfv1.ParseAnyString("")
	stage := wfv1.ParseAnyString(d.Stage)
	isNewProjectAS := wfv1.ParseAnyString(fmt.Sprintf("%t", d.IsNewProject))

	if d.BasePath != nil {
		basePathAS = wfv1.ParseAnyString(*d.BasePath)
	}

	if d.Stack != nil {
		jsonData, err := json.Marshal(d.Stack)
		if err != nil {
			log.Println("Error marshalling stack", d.Stack)
		} else {
			stackAS = wfv1.ParseAnyString(string(jsonData))
		}
	}
	log.Printf("stackAS = %v", stackAS)

	templateName := "build-git"
	templateRef := "genezio-build-git-template"
	generateName := "genezio-build-git-"
	if internal.GetConfig().Env == "dev" || internal.GetConfig().Env == "local" {
		templateName = "build-git-dev"
		templateRef = "genezio-build-git-template-dev"
		generateName = "genezio-build-git-dev-"
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
												Name:  "token",
												Value: &tokenAS,
											},
											{
												Name:  "githubRepository",
												Value: &repoAS,
											},
											{
												Name:  "region",
												Value: &regionAS,
											},
											{
												Name:  "projectName",
												Value: &projectnameAS,
											},
											{
												Name:  "basePath",
												Value: &basePathAS,
											},
											{
												Name:  "stack",
												Value: &stackAS,
											},
											{
												Name:  "isNewProject",
												Value: &isNewProjectAS,
											},
											{
												Name:  "stage",
												Value: &stage,
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
