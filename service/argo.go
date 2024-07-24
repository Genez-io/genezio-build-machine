package service

import (
	"build-machine/internal"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os/user"
	"path/filepath"
	"time"

	wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	wfclientset "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned/typed/workflow/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
)

type ArgoService struct {
	wfClient v1alpha1.WorkflowInterface
	k8Client *kubernetes.Clientset
	config   *rest.Config
}

func NewArgoService() *ArgoService {
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
	return &ArgoService{
		wfClient: wfClient,
		k8Client: clientSet,
		config:   config,
	}
}

func (w *ArgoService) SubmitWorkflow(workflowRender wfv1.Workflow) (string, error) {
	ctx := context.Background()
	createdWf, err := w.wfClient.Create(ctx, &workflowRender, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}
	fmt.Printf("Workflow %s submitted\n", createdWf.Name)
	return createdWf.Name, nil
}

// {"status":"PENDING","message":"Starting build from git flow","time":"2024-07-17T17:35:45.988Z"}
type ArgoPodStatus struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Time    string `json:"time"`
}

func (w *ArgoService) ReadStatusFileFromPod(jobId string) ([]ArgoPodStatus, error) {
	time.Sleep(time.Millisecond * 500)
	pods, err := w.k8Client.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("workflows.argoproj.io/workflow=%s", jobId),
	})

	if err != nil {
		return nil, err
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no pod found for workflow %s", jobId)
	}
	workflowPod := pods.Items[0]
	if workflowPod.Status.Phase == v1.PodSucceeded || workflowPod.Status.Phase == v1.PodFailed {
		workflowRef, err := w.wfClient.Get(context.Background(), jobId, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		if workflowPod.Status.Phase == v1.PodSucceeded {
			return []ArgoPodStatus{
				{
					Status:  "SUCCEEDED",
					Message: "Workflow completed successfully",
					Time:    time.Now().String(),
				},
			}, nil
		} else if workflowPod.Status.Phase == v1.PodFailed {
			return []ArgoPodStatus{
				{
					Status:  "FAILED",
					Message: fmt.Sprintln(workflowRef.Status.Outputs),
					Time:    time.Now().String(),
				},
			}, nil
		}
	}
	if workflowPod.Status.Phase != v1.PodRunning {
		return []ArgoPodStatus{}, fmt.Errorf("pod %s is not yet running", workflowPod.Name)
	}

	// Extract status.json from pod
	containerName := "main"
	cmd := []string{"cat", "/tmp/status.json"}
	option := &v1.PodExecOptions{
		Command:   cmd,
		Stdout:    true,
		Stderr:    true,
		Container: containerName,
	}
	req := w.k8Client.CoreV1().RESTClient().Post().Resource("pods").Name(pods.Items[0].Name).Namespace("default").SubResource("exec").VersionedParams(
		option,
		scheme.ParameterCodec,
	)
	exec, err := remotecommand.NewSPDYExecutor(w.config, "POST", req.URL())
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	stdout_buf := &bytes.Buffer{}
	stderr_buf := &bytes.Buffer{}
	err = exec.StreamWithContext(context.Background(), remotecommand.StreamOptions{
		Stdout: stdout_buf,
		Stderr: stderr_buf,
	})
	if err != nil {
		fmt.Println("de aici", err.Error(), stderr_buf.String(), stdout_buf.String())
		return nil, err
	}
	stdout_res := make([]byte, stdout_buf.Len())
	stdout_buf.Read(stdout_res)
	// marshal to state array
	var states []ArgoPodStatus
	log.Printf("stdout_res = %v", string(stdout_res))
	err = json.Unmarshal(stdout_res, &states)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	return states, nil
}

func checkErr(err error) {
	if err != nil {
		panic(err.Error())
	}
}
