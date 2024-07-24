package workflows

import (
	"build-machine/internal"
	"build-machine/service"
	statemanager "build-machine/state_manager"
	"encoding/json"
	"fmt"
	"log"
	"slices"

	wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GitDeploymentArgo struct {
	GitDeployment
	Token        string
    Stage        string
	ArgoClient   service.ArgoService
	StateManager statemanager.StateManager
}

// AssignStateManager implements Workflow.
func (d *GitDeploymentArgo) AssignStateManager(state statemanager.StateManager) {
	d.StateManager = state
}

// GetState implements Workflow.
func (d *GitDeploymentArgo) GetState() (WorkflowReport, error) {
	panic("unimplemented")
}

// Submit implements Workflow.
func (d *GitDeploymentArgo) Submit() (string, error) {
	renderedWorkflow := d.RenderArgoTemplate()
	wf_id, err := d.ArgoClient.SubmitWorkflow(renderedWorkflow)
	if err != nil {
		return "", err
	}

	err = d.StateManager.CreateState(wf_id, d.Token, "argo")
	if err != nil {
		return "", err
	}

	go func() {
		maxRetries := 35
		for {
			// In the future we should have a better way to handle this
			// For now we will just poll the status of the workflow
			// A high number of retries is needed in case of delayed scheduling on the cluster
			if maxRetries == 0 {
				break
			}
			res, err := d.ArgoClient.ReadStatusFileFromPod(wf_id)
			if err != nil {
				fmt.Println(err)
				maxRetries--
				continue
			}

            log.Printf("Workflow %s status: %v", wf_id, res)

			// get current state history
			state, err := d.StateManager.GetState(wf_id)
			if err != nil {
				fmt.Println(err)
				break
			}

			for _, retrievedState := range res {
				seenThisState := slices.ContainsFunc(state.Transitions, func(i statemanager.StateTransition) bool {
					return retrievedState.Status == string(i.From) || retrievedState.Status == string(i.To)
				})

				if !seenThisState {
					d.StateManager.UpdateState(wf_id, retrievedState.Message, statemanager.BuildStatus(retrievedState.Status))
				}

				if retrievedState.Status == "SUCCEEDED" || retrievedState.Status == "FAILED" {
					return
				}
			}
		}
	}()
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

	if d.ProjectName == "" {
		return fmt.Errorf("projectName is required")
	}

	if d.Region == "" {
		return fmt.Errorf("region is required")
	}

	return nil
}

func NewGitArgoWorkflow(token string, stage string) Workflow {
	argoService := service.NewArgoService()

	return &GitDeploymentArgo{
		Token:      token,
        Stage:      stage,
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
