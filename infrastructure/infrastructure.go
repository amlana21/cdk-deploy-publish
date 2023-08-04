package main

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudfront"
	origins "github.com/aws/aws-cdk-go/awscdk/v2/awscloudfrontorigins"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecr"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	elb "github.com/aws/aws-cdk-go/awscdk/v2/awselasticloadbalancingv2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

type InfrastructureStackProps struct {
	awscdk.StackProps
}

func NewInfrastructureStack(scope constructs.Construct, id string, props *InfrastructureStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	//get the current account and region
	account_id := awscdk.Aws_ACCOUNT_ID()
	account_id_str := *account_id
	// Create a VPC with two public and two private subnets
	vpc := awsec2.NewVpc(stack, jsii.String("pdfappvpc"), &awsec2.VpcProps{
		Cidr:   jsii.String("10.0.0.0/16"),
		MaxAzs: jsii.Number(2),
		SubnetConfiguration: &[]*awsec2.SubnetConfiguration{
			{
				CidrMask:   jsii.Number(24),
				Name:       jsii.String("Public"),
				SubnetType: awsec2.SubnetType_PUBLIC,
			},
			{
				CidrMask:   jsii.Number(24),
				Name:       jsii.String("Private"),
				SubnetType: awsec2.SubnetType_PRIVATE_WITH_NAT,
			},
		},
	})

	// security group for the load balancer
	lbSecurityGroup := awsec2.NewSecurityGroup(stack, jsii.String("pdfappalbsg"), &awsec2.SecurityGroupProps{
		Vpc: vpc,
	})
	// add ingress rule to allow port 80
	lbSecurityGroup.AddIngressRule(awsec2.Peer_AnyIpv4(), awsec2.Port_Tcp(jsii.Number(80)), jsii.String("allow public http access"), jsii.Bool(false))
	lbSecurityGroup.AddIngressRule(awsec2.Peer_AnyIpv4(), awsec2.Port_Tcp(jsii.Number(3000)), jsii.String("allow public http access"), jsii.Bool(false))
	lbSecurityGroup.AddIngressRule(awsec2.Peer_AnyIpv4(), awsec2.Port_Tcp(jsii.Number(8501)), jsii.String("allow public http access"), jsii.Bool(false))
	lbSecurityGroup.AddIngressRule(awsec2.Peer_AnyIpv4(), awsec2.Port_Tcp(jsii.Number(5000)), jsii.String("allow public http access"), jsii.Bool(false))

	// create ecr repository
	repo := awsecr.NewRepository(stack, jsii.String("pdfapprepo"), &awsecr.RepositoryProps{
		RepositoryName: jsii.String("pdfapprepo"),
	})

	//create ecs cluster
	cluster := awsecs.NewCluster(stack, jsii.String("pdfappcluster"), &awsecs.ClusterProps{
		Vpc: vpc,
	})

	taskExecutionRole := awsiam.NewRole(stack, jsii.String("pdfapptaskexecutionrole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("ecs-tasks.amazonaws.com"), &awsiam.ServicePrincipalOpts{}),
		ManagedPolicies: &[]awsiam.IManagedPolicy{
			awsiam.ManagedPolicy_FromManagedPolicyArn(stack, jsii.String("ECSTaskExecRole"), jsii.String("arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy")),
		},
	})

	// Create Task Definition
	taskDef := awsecs.NewFargateTaskDefinition(stack, jsii.String("pdfapptask"), &awsecs.FargateTaskDefinitionProps{
		Cpu:            jsii.Number(256),
		MemoryLimitMiB: jsii.Number(512),
		// TaskRole:       taskExecutionRole,
		ExecutionRole: taskExecutionRole,
	})

	repo_name := repo.RepositoryName()
	repo_name_str := *repo_name
	// concatenate the account number and region to create the image name
	image_name := jsii.String(fmt.Sprintf("%s.dkr.ecr.us-east-1.amazonaws.com/%s:latest", account_id_str, repo_name_str))
	container := taskDef.AddContainer(jsii.String("pdfapptaskFargoContainer"), &awsecs.ContainerDefinitionOptions{
		Image: awsecs.ContainerImage_FromRegistry(image_name, &awsecs.RepositoryImageProps{}),
	})
	container.AddPortMappings(&awsecs.PortMapping{
		ContainerPort: jsii.Number(8501),
		Protocol:      awsecs.Protocol_TCP,
	})

	// create ecs service
	service := awsecs.NewFargateService(stack, jsii.String("pdfappservice"), &awsecs.FargateServiceProps{
		Cluster:        cluster,
		TaskDefinition: taskDef,
		AssignPublicIp: jsii.Bool(true),
		DesiredCount:   jsii.Number(1),
		SecurityGroups: &[]awsec2.ISecurityGroup{
			lbSecurityGroup,
		},
		VpcSubnets: &awsec2.SubnetSelection{
			SubnetType: awsec2.SubnetType_PRIVATE_WITH_NAT,
		},
	})

	// create application load balancer
	lb := elb.NewApplicationLoadBalancer(stack, jsii.String("pdfappalb"), &elb.ApplicationLoadBalancerProps{
		Vpc:            vpc,
		InternetFacing: jsii.Bool(true),
		VpcSubnets: &awsec2.SubnetSelection{
			SubnetType: awsec2.SubnetType_PUBLIC,
		},
		SecurityGroup: lbSecurityGroup,
	})

	listener := lb.AddListener(jsii.String("pdflistenere"), &elb.BaseApplicationListenerProps{
		Port: jsii.Number(80),
		Open: jsii.Bool(true),
	})
	// Attach ALB to Fargate Service
	listener.AddTargets(jsii.String("pdfapptarget"), &elb.AddApplicationTargetsProps{
		Port: jsii.Number(80),
		Targets: &[]elb.IApplicationLoadBalancerTarget{
			service.LoadBalancerTarget(&awsecs.LoadBalancerTargetOptions{
				ContainerName: jsii.String("pdfapptaskFargoContainer"),
				ContainerPort: jsii.Number(8501),
			}),
		},
		HealthCheck: &elb.HealthCheck{
			Interval: awscdk.Duration_Seconds(jsii.Number(60)),
			Path:     jsii.String("/"),
			Port:     jsii.String("8501"),
		},
	})

	// create cloudfront for the load balancer
	cloudfrontDefaultBehavior := &awscloudfront.BehaviorOptions{
		Origin: origins.NewLoadBalancerV2Origin(lb, &origins.LoadBalancerV2OriginProps{
			ProtocolPolicy: awscloudfront.OriginProtocolPolicy_HTTP_ONLY,
		}),
		Compress:             jsii.Bool(true),
		AllowedMethods:       awscloudfront.AllowedMethods_ALLOW_ALL(),
		ViewerProtocolPolicy: awscloudfront.ViewerProtocolPolicy_ALLOW_ALL,
		OriginRequestPolicy: awscloudfront.NewOriginRequestPolicy(stack, jsii.String("pdfapporiginrequestpolicy"), &awscloudfront.OriginRequestPolicyProps{
			Comment:             jsii.String("pdfapporiginrequestpolicy"),
			CookieBehavior:      awscloudfront.OriginRequestCookieBehavior_All(),
			HeaderBehavior:      awscloudfront.OriginRequestHeaderBehavior_All(),
			QueryStringBehavior: awscloudfront.OriginRequestQueryStringBehavior_All(),
		}),
	}

	awscloudfront.NewDistribution(stack, jsii.String("pdfapp"), &awscloudfront.DistributionProps{
		DefaultBehavior: cloudfrontDefaultBehavior,
	})

	return stack
}

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)

	NewInfrastructureStack(app, "InfrastructureStack", &InfrastructureStackProps{
		awscdk.StackProps{
			Env: env(),
		},
	})

	app.Synth(nil)
}

// env determines the AWS environment (account+region) in which our stack is to
// be deployed. For more information see: https://docs.aws.amazon.com/cdk/latest/guide/environments.html
func env() *awscdk.Environment {
	// If unspecified, this stack will be "environment-agnostic".
	// Account/Region-dependent features and context lookups will not work, but a
	// single synthesized template can be deployed anywhere.
	//---------------------------------------------------------------------------
	return nil

	// Uncomment if you know exactly what account and region you want to deploy
	// the stack to. This is the recommendation for production stacks.
	//---------------------------------------------------------------------------
	// return &awscdk.Environment{
	//  Account: jsii.String("123456789012"),
	//  Region:  jsii.String("us-east-1"),
	// }

	// Uncomment to specialize this stack for the AWS Account and Region that are
	// implied by the current CLI configuration. This is recommended for dev
	// stacks.
	//---------------------------------------------------------------------------
	// return &awscdk.Environment{
	//  Account: jsii.String(os.Getenv("CDK_DEFAULT_ACCOUNT")),
	//  Region:  jsii.String(os.Getenv("CDK_DEFAULT_REGION")),
	// }
}
