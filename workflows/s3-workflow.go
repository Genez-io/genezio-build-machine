package workflows

import (
	wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func S3Workflow(token, s3URL string) wfv1.Workflow {
	tokenAS := wfv1.ParseAnyString(token)
	s3URLAS := wfv1.ParseAnyString(s3URL)
	s3FilePerms := int32(0755)
	return wfv1.Workflow{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "genezio-build-s3-",
		},
		Spec: wfv1.WorkflowSpec{
			Entrypoint:         "build-s3",
			ServiceAccountName: "argo-workflow",
			Templates: []wfv1.Template{
				{
					Name: "build-s3",
					Steps: []wfv1.ParallelSteps{
						{
							Steps: []wfv1.WorkflowStep{
								{
									Name: "genezio-deploy",
									TemplateRef: &wfv1.TemplateRef{
										Name:     "genezio-build-s3-template",
										Template: "build-s3",
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
