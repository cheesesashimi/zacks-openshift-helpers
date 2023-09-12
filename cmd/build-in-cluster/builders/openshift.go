package builders

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	buildv1 "github.com/openshift/api/build/v1"
	ctrlcommon "github.com/openshift/machine-config-operator/pkg/controller/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/machine-config-operator/test/framework"
)

type OpenshiftBuilderOpts struct {
	Opts
	ImageStreamPullspec string
	Dockerfile          string
	BranchName          string
	RemoteURL           string
}

type OpenshiftBuilder struct {
	opts OpenshiftBuilderOpts
	cs   *framework.ClientSet
}

func NewOpenshiftBuilder(cs *framework.ClientSet, opts OpenshiftBuilderOpts) *OpenshiftBuilder {
	return &OpenshiftBuilder{
		opts: opts,
		cs:   cs,
	}
}

func (o *OpenshiftBuilder) Build() (string, error) {
	_, err := o.cs.BuildV1Interface.Builds(ctrlcommon.MCONamespace).Create(context.TODO(), o.prepareBuild(), metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("could not create build: %w", err)
	}

	if err := o.waitForBuildToComplete(); err != nil {
		return "", err
	}

	return o.getImagePullspec(), nil
}

func (o *OpenshiftBuilder) getImagePullspec() string {
	return o.opts.ImageStreamPullspec + ":latest"
}

func (o *OpenshiftBuilder) waitForBuildToComplete() error {
	cmd := exec.Command("oc", "logs", "-f", "build/mco-image-build", "-n", ctrlcommon.MCONamespace)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (o *OpenshiftBuilder) prepareBuild() *buildv1.Build {
	skipLayers := buildv1.ImageOptimizationSkipLayers

	//    return {
	//        "apiVersion": "build.openshift.io/v1",
	//        "kind": "Build",
	//        "metadata": {
	//            "name": "mco-image-build",
	//            "namespace": MCO_NAMESPACE,
	//        },
	//        "spec": {
	//            "output": {
	//                "to": {
	//                    "kind": "ImageStreamTag",
	//                    "name": get_mco_imagestream_spec()["metadata"]["name"] + ":latest",
	//                }
	//            },
	//            "postCommit": {},
	//            "serviceAccount": "builder",
	//            "source": {
	//                # Delete this line once https://issues.redhat.com/browse/MCO-603 is resolved
	//                "dockerfile": dockerfile,
	//                "git": {
	//                    "uri": get_git_remote(),
	//                    "ref": get_git_branch(),
	//                },
	//                "type": "Dockerfile"
	//            },
	//            "strategy": {
	//                "dockerStrategy": {},
	//                "type": "Docker"
	//            }
	//        }
	//    }

	return &buildv1.Build{
		TypeMeta: metav1.TypeMeta{
			Kind: "Build",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ctrlcommon.MCONamespace,
			Name:      "mco-image-build",
		},
		Spec: buildv1.BuildSpec{
			CommonSpec: buildv1.CommonSpec{
				Source: buildv1.BuildSource{
					Type:       buildv1.BuildSourceDockerfile,
					Dockerfile: &o.opts.Dockerfile,
					Git: &buildv1.GitBuildSource{
						URI: o.opts.RemoteURL,
						Ref: o.opts.BranchName,
					},
				},
				Strategy: buildv1.BuildStrategy{
					DockerStrategy: &buildv1.DockerBuildStrategy{
						// Squashing layers is good as long as it doesn't cause problems with what
						// the users want to do. It says "some syntax is not supported"
						ImageOptimizationPolicy: &skipLayers,
					},
					Type: buildv1.DockerBuildStrategyType,
				},
				Output: buildv1.BuildOutput{
					To: &corev1.ObjectReference{
						Name: o.getImagePullspec(),
						Kind: "ImageStreamTag",
					},
				},
			},
		},
	}
}
