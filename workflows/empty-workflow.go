package workflows

import (
	wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func EmptyWorkflow(token, repo, region, projectname string, basePath *string, stack []string) wfv1.Workflow {
	tokenAS := wfv1.ParseAnyString(token)
	repoAS := wfv1.ParseAnyString(repo)
	regionAS := wfv1.ParseAnyString(region)
	projectnameAS := wfv1.ParseAnyString(projectname)
	basePathAS := wfv1.ParseAnyString(basePath)

	if basePath == nil {
		basePathAS = wfv1.ParseAnyString("")
	}
	return wfv1.Workflow{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "genezio-build-empty-",
		},
		Spec: wfv1.WorkflowSpec{
			Entrypoint:         "build-empty",
			ServiceAccountName: "argo-workflow",
			Templates: []wfv1.Template{
				{
					Name: "build-empty",
					Steps: []wfv1.ParallelSteps{
						{
							Steps: []wfv1.WorkflowStep{
								{
									Name: "genezio-deploy",
									TemplateRef: &wfv1.TemplateRef{
										Name:     "genezio-build-empty-template",
										Template: "build-empty",
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
