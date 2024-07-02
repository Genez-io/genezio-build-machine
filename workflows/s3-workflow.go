package workflows

import (
	"build-machine/internal"

	wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func S3Workflow(token, s3URL string) wfv1.Workflow {
	tokenAS := wfv1.ParseAnyString(token)
	s3URLAS := wfv1.ParseAnyString(s3URL)
	s3FilePerms := int32(0755)

	templateName := "build-s3"
	templateRef := "genezio-build-s3-template"
	generateName := "genezio-build-s3-"
	if internal.GetConfig().Env == "dev" || internal.GetConfig().Env == "local" {
		templateName = "build-s3-dev"
		templateRef = "genezio-build-s3-template-dev"
		generateName = "genezio-build-s3-dev-"
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
										Artifacts: []wfv1.Artifact{
											{
												Name: "codeArchive",
												Path: "/tmp/projectCode.zip",
												Mode: &s3FilePerms,
												ArtifactLocation: wfv1.ArtifactLocation{
													HTTP: &wfv1.HTTPArtifact{
														URL: s3URL,
													},
												},
											},
										},
										Parameters: []wfv1.Parameter{
											{
												Name:  "token",
												Value: &tokenAS,
											},
											{
												Name:  "codeURL",
												Value: &s3URLAS,
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
