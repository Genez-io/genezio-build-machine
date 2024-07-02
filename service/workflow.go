package service

import (
	"build-machine/internal"
	"build-machine/workflows"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os/user"
	"path/filepath"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/pointer"

	wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	wfclientset "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned/typed/workflow/v1alpha1"
	"github.com/argoproj/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type WorkflowReport struct {
	WorkflowName     string
	Phase            string
	FinishedAt       metav1.Time
	Message          string
	ResourceDuration string
}

type WorkflowService interface {
	// S3Workflow is a function that runs a workflow with input code from S3
	S3Workflow(token, S3URL, projectName, region string, stage, basePath *string) string
	// GithubWorkflow is a function that runs a workflow with input code from Github
	GithubWorkflow(token, githubRepository string, projectName, region, basePath *string) string
	// EmptyProjectWorkflow is a function that runs a workflow with no input code
	EmptyProjectWorkflow(token, githubRepository string, projectName, region, basePath *string, stack []string) string
}

type workflowService struct {
	client v1alpha1.WorkflowInterface
}

func NewWorkflowService() WorkflowService {
	// get current user to determine home directory
	usr, err := user.Current()
	checkErr(err)
	kubeconfigDir := filepath.Join(usr.HomeDir, ".kube", "config")
	processENV := internal.GetConfig().Env
	var wfClient v1alpha1.WorkflowInterface
	var config *rest.Config
	namespace := "default"
	if processENV == "local" {
		// get kubeconfig file location
		kubeconfig := flag.String("kubeconfig", kubeconfigDir, "(optional) absolute path to the kubeconfig file")
		flag.Parse()

		// use the current context in kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		checkErr(err)
	} else {
		config = NewKubernetesConfig().Config
	}
	wfClient = wfclientset.NewForConfigOrDie(config).ArgoprojV1alpha1().Workflows(namespace)

	return &workflowService{
		client: wfClient,
	}
}

// EmptyProjectWorkflow implements WorkflowService.
func (w *workflowService) EmptyProjectWorkflow(token, githubRepository string, projectName, region, basePath *string, stack []string) string {
	renderedWorkflow := workflows.EmptyWorkflow(token, githubRepository, *region, *projectName, basePath, stack)
	report, err := w.submitWorkflow(renderedWorkflow)

	if err != nil {
		return err.Error()
	}

	// Pretty print json report
	reportBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err.Error()
	}

	fmt.Println(string(reportBytes))
	return string(reportBytes)
}

// GithubWorkflow implements WorkflowService.
func (w *workflowService) GithubWorkflow(token string, githubRepository string, projectName, region, basePath *string) string {
	renderedWorkflow := workflows.GitWorkflow(token, githubRepository, *region, *projectName, basePath)
	report, err := w.submitWorkflow(renderedWorkflow)

	if err != nil {
		return err.Error()
	}

	// Pretty print json report
	reportBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err.Error()
	}

	fmt.Println(string(reportBytes))
	return string(reportBytes)
}

// S3Workflow implements WorkflowService.
func (w *workflowService) S3Workflow(token, S3URL, projectName, region string, stage, basePath *string) string {
	renderedWorkflow := workflows.S3Workflow(token, S3URL)
	report, err := w.submitWorkflow(renderedWorkflow)

	if err != nil {
		return err.Error()
	}

	// Pretty print json report
	reportBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err.Error()
	}

	fmt.Println(string(reportBytes))
	return string(reportBytes)
}

func checkErr(err error) {
	if err != nil {
		panic(err.Error())
	}
}

func (w *workflowService) submitWorkflow(workflowRender wfv1.Workflow) (WorkflowReport, error) {
	ctx := context.Background()
	createdWf, err := w.client.Create(ctx, &workflowRender, metav1.CreateOptions{})
	checkErr(err)
	fmt.Printf("Workflow %s submitted\n", createdWf.Name)

	// wait for the workflow to complete
	fieldSelector := fields.ParseSelectorOrDie(fmt.Sprintf("metadata.name=%s", createdWf.Name))
	watchIf, err := w.client.Watch(ctx, metav1.ListOptions{FieldSelector: fieldSelector.String(), TimeoutSeconds: pointer.Int64(180)})
	errors.CheckError(err)
	defer watchIf.Stop()
	for next := range watchIf.ResultChan() {
		wf, ok := next.Object.(*wfv1.Workflow)
		if !ok {
			continue
		}
		if wf.Status.Failed() {
			return WorkflowReport{}, fmt.Errorf("workflow %s %s at %v. Message: %s", wf.Name, wf.Status.Phase, wf.Status.FinishedAt, wf.Status.Message)
		}

		if !wf.Status.FinishedAt.IsZero() {
			fmt.Printf("Workflow %s %s at %v. Message: %s.\n", wf.Name, wf.Status.Phase, wf.Status.FinishedAt, wf.Status.Message)
			parseResourceDuration := make(map[string]string)
			for k, v := range wf.Status.ResourcesDuration {
				parseResourceDuration[string(k)] = v.String()
			}

			return WorkflowReport{
				WorkflowName:     wf.Name,
				Phase:            string(wf.Status.Phase),
				FinishedAt:       wf.Status.FinishedAt,
				Message:          wf.Status.Message,
				ResourceDuration: wf.Status.ResourcesDuration.String(),
			}, nil
		}
	}

	return WorkflowReport{}, nil
}
