package workflows

import (
	"build-machine/internal"

	wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// - name: token
// - name: githubRepository
// - name: region
// - name: projectName
// - name: basePath
func GitWorkflow(token, repo, region, projectname string, basePath *string) wfv1.Workflow {
	tokenAS := wfv1.ParseAnyString(token)
	repoAS := wfv1.ParseAnyString(repo)
	regionAS := wfv1.ParseAnyString(region)
	projectnameAS := wfv1.ParseAnyString(projectname)
	basePathAS := wfv1.ParseAnyString("")

	if basePath != nil {
		basePathAS = wfv1.ParseAnyString(*basePath)
	}

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
