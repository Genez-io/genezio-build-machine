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

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	wfclientset "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned/typed/workflow/v1alpha1"
	"github.com/argoproj/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type WorkflowReport struct {
	JobName   string
	Status    string
	StartTime string
	EndTime   string
	Message   string
}

type WorkflowService interface {
	// S3Workflow is a function that runs a workflow with input code from S3
	S3Workflow(token, S3URL, projectName, region string, stage, basePath *string) string
	// GithubWorkflow is a function that runs a workflow with input code from Github
	GithubWorkflow(token, githubRepository string, projectName, region, basePath *string) string
	// EmptyProjectWorkflow is a function that runs a workflow with no input code
	EmptyProjectWorkflow(token, githubRepository string, projectName, region, basePath *string, stack *string) string
}

type workflowService struct {
	wfClient v1alpha1.WorkflowInterface
	k8Client *kubernetes.Clientset
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
	clientSet, err := kubernetes.NewForConfig(config)
	checkErr(err)
	return &workflowService{
		wfClient: wfClient,
		k8Client: clientSet,
	}
}

// EmptyProjectWorkflow implements WorkflowService.
func (w *workflowService) EmptyProjectWorkflow(token, githubRepository string, projectName, region, basePath *string, stack *string) string {
	renderedWorkflow := workflows.EmptyWorkflow(token, githubRepository, *region, *projectName, basePath, stack)
	report, err_wf := w.submitWorkflow(renderedWorkflow)
	reportBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err.Error()
	}
	if err_wf != nil {
		return string(reportBytes)
	}

	// Pretty print json report
	fmt.Println(string(reportBytes))
	return string(reportBytes)
}

// GithubWorkflow implements WorkflowService.
func (w *workflowService) GithubWorkflow(token string, githubRepository string, projectName, region, basePath *string) string {
	renderedWorkflow := workflows.GitWorkflow(token, githubRepository, *region, *projectName, basePath)
	report, err_wf := w.submitWorkflow(renderedWorkflow)
	reportBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err.Error()
	}
	if err_wf != nil {
		return string(reportBytes)
	}

	// Pretty print json report
	fmt.Println(string(reportBytes))
	return string(reportBytes)
}

// S3Workflow implements WorkflowService.
func (w *workflowService) S3Workflow(token, S3URL, projectName, region string, stage, basePath *string) string {
	renderedWorkflow := workflows.S3Workflow(token, S3URL)
	report, err_wf := w.submitWorkflow(renderedWorkflow)
	reportBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err.Error()
	}
	if err_wf != nil {
		return string(reportBytes)
	}

	// Pretty print json report
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
	createdWf, err := w.wfClient.Create(ctx, &workflowRender, metav1.CreateOptions{})
	checkErr(err)
	fmt.Printf("Workflow %s submitted\n", createdWf.Name)

	// list pod with workflow name
	podIf, err := w.k8Client.CoreV1().Pods("default").Watch(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("workflows.argoproj.io/workflow=%s", createdWf.Name)})
	errors.CheckError(err)

	for next := range podIf.ResultChan() {
		if next.Object == nil {
			continue
		}
		pod, ok := next.Object.(*v1.Pod)
		if !ok {
			continue
		}

		if pod.Status.Phase == v1.PodFailed {
			return WorkflowReport{
				JobName:   createdWf.Name,
				Status:    string(pod.Status.Phase),
				StartTime: pod.Status.StartTime.String(),
				EndTime:   metav1.Now().String(),
				Message:   fmt.Sprintf("workflow %s %s at %v. Message: %s", createdWf.Name, pod.Status.Phase, pod.Status.StartTime, pod.Status.Message),
			}, fmt.Errorf("workflow %s %s at %v. Message: %s", createdWf.Name, pod.Status.Phase, pod.Status.StartTime, pod.Status.Message)
		}

		if pod.Status.Phase == v1.PodSucceeded {
			timeNow := metav1.Now()
			return WorkflowReport{
				JobName:   createdWf.Name,
				Status:    string(pod.Status.Phase),
				StartTime: pod.Status.StartTime.String(),
				EndTime:   timeNow.String(),
				Message:   fmt.Sprintf("workflow %s %s at %v. Message: %s", createdWf.Name, pod.Status.Phase, pod.Status.StartTime, pod.Status.Message),
			}, nil
		}
	}

	return WorkflowReport{}, nil
}
