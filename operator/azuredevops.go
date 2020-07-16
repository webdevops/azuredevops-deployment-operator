package operator

import (
	"context"
	"fmt"
	"github.com/containrrr/shoutrrr"
	"github.com/microsoft/azure-devops-go-api/azuredevops"
	"github.com/microsoft/azure-devops-go-api/azuredevops/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/release"
	"github.com/prometheus/client_golang/prometheus"
	cron "github.com/robfig/cron/v3"
	log "github.com/sirupsen/logrus"
	"github.com/webdevops/azuredevops-deployment-operator/config"
	"strings"
	"time"
)

var (
	releaseTriggerCancelComment = "deployment canceled webdevops/azuredevops-deployment-operator"
	releaseTriggerNewComment    = "deployment triggered by webdevops/azuredevops-deployment-operator"
	releaseAutoApprovalComment  = "auto-approval by webdevops/azuredevops-deployment-operator"

	timeWaitDeploy = 5 * time.Second
)

type (
	AzureDevopsOperator struct {
		Opts         config.OptsAzureDevops
		Notification config.OptsNotification
		Config       config.Config

		ctx        context.Context
		connection *azuredevops.Connection

		cron *cron.Cron

		coreClient    core.Client
		releaseClient release.Client

		prometheus struct {
			deploymentCounter *prometheus.CounterVec
			deploymentStatus  *prometheus.GaugeVec
			deploymentTime    *prometheus.GaugeVec
		}
	}
)

func (o *AzureDevopsOperator) Init() {
	o.ctx = context.Background()
	o.initAzureDevops()
	o.initCron()
	o.initMetrics()
}

func (o *AzureDevopsOperator) initAzureDevops() {
	o.connection = azuredevops.NewPatConnection(o.Opts.OrganizationUrl, o.Opts.AccessToken)

	// core client (projects,...)
	coreClient, err := core.NewClient(o.ctx, o.connection)
	if err != nil {
		log.Error(err)
	}
	o.coreClient = coreClient

	// release clients (releases, environments, definitions,...)
	releaseClient, err := release.NewClient(o.ctx, o.connection)
	if err != nil {
		log.Error(err)
	}
	o.releaseClient = releaseClient
}

func (o *AzureDevopsOperator) initCron() {
	o.cron = cron.New()
}

func (o *AzureDevopsOperator) initMetrics() {
	o.prometheus.deploymentCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "azuredevops_deployment_operator_deployment_counter",
			Help: "AzureDevOps deployment operator: last deployment counter",
		},
		[]string{"projectId", "projectName", "releaseDefinitionName", "environmentName"},
	)
	prometheus.MustRegister(o.prometheus.deploymentCounter)

	o.prometheus.deploymentStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azuredevops_deployment_operator_deployment_status",
			Help: "AzureDevOps deployment operator: last deployment status",
		},
		[]string{"projectId", "projectName", "releaseDefinitionName", "environmentName"},
	)
	prometheus.MustRegister(o.prometheus.deploymentStatus)

	o.prometheus.deploymentTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azuredevops_deployment_operator_deployment_time",
			Help: "AzureDevOps deployment operator: last deployment time",
		},
		[]string{"projectId", "projectName", "releaseDefinitionName", "environmentName"},
	)
	prometheus.MustRegister(o.prometheus.deploymentTime)
}

func (o *AzureDevopsOperator) Start() {
	for _, config := range o.Config.Releases {
		_, err := o.cron.AddFunc(config.Crontab, func() {
			contextLogger := log.WithFields(log.Fields{
				"type":    "release",
				"project": *config.Project,
				"crontab": config.Crontab,
			})
			o.triggerReleaseDefinitions(contextLogger, config)
		})

		if err != nil {
			log.Panic(err)
		}
	}

	o.cron.Start()
}

func (o *AzureDevopsOperator) triggerReleaseDefinitions(contextLogger *log.Entry, config config.ConfigRelease) {
	projArgs := core.GetProjectArgs{
		ProjectId: config.Project,
	}
	project, err := o.coreClient.GetProject(o.ctx, projArgs)
	if err != nil {
		contextLogger.Error(err)
	}

	expand := release.ReleaseDefinitionExpands(
		strings.Join([]string{
			string(release.ReleaseDefinitionExpandsValues.Environments),
			string(release.ReleaseDefinitionExpandsValues.LastRelease),
		}, ","),
	)
	args := release.GetReleaseDefinitionsArgs{
		Project:                      config.Project,
		SearchText:                   config.ReleaseDefinitions.SearchText,
		ArtifactType:                 config.ReleaseDefinitions.ArtifactType,
		ArtifactSourceId:             config.ReleaseDefinitions.ArtifactSourceId,
		Path:                         config.ReleaseDefinitions.Path,
		IsExactNameMatch:             config.ReleaseDefinitions.IsExactNameMatch,
		TagFilter:                    config.ReleaseDefinitions.TagFilter,
		DefinitionIdFilter:           config.ReleaseDefinitions.DefinitionIdFilter,
		SearchTextContainsFolderName: config.ReleaseDefinitions.SearchTextContainsFolderName,
		Expand:                       &expand,
	}

	releaseDefinitionList, err := o.releaseClient.GetReleaseDefinitions(o.ctx, args)
	if err != nil {
		contextLogger.Error(err)
	}

	for _, releaseDefinition := range releaseDefinitionList.Value {
		// inject project (it's empty when fetching from the api)
		releaseDefinition.ProjectReference = &release.ProjectReference{
			Id:   project.Id,
			Name: project.Name,
		}

		err := o.triggerReleaseDefinitionDeploy(contextLogger, config, &releaseDefinition)
		if err != nil {
			contextLogger.Error(err)
		}
	}
}

func (o *AzureDevopsOperator) triggerReleaseDefinitionDeploy(contextLogger *log.Entry, config config.ConfigRelease, releaseDefinition *release.ReleaseDefinition) error {
	contextLogger = contextLogger.WithField("releaseDefinition", o.buildReleaseDefinitionName(releaseDefinition))
	contextLogger.Infof("starting deployment")
	// check if there are environments defined
	if releaseDefinition.Environments == nil {
		return nil
	}

	// reset metrics
	for _, environment := range config.Environments {
		promLabels := prometheus.Labels{
			"projectId":             releaseDefinition.ProjectReference.Id.String(),
			"projectName":           *releaseDefinition.ProjectReference.Name,
			"releaseDefinitionName": o.buildReleaseDefinitionName(releaseDefinition),
			"environmentName":       environment,
		}
		o.prometheus.deploymentStatus.With(promLabels).Set(0)
	}

	for _, environment := range *releaseDefinition.Environments {
		// only check selected environments
		if stringArrayContains(config.Environments, *environment.Name) {
			environmentLogger := contextLogger.WithField("environment", *environment.Name)
			switch config.Trigger {
			// deploy latest version
			case "latest":
				if releaseDefinition.LastRelease != nil && releaseDefinition.LastRelease.Id != nil {
					environmentLogger.Info("trigger latest release")
					err := o.triggerExistingReleaseDeployment(
						environmentLogger,
						releaseDefinition,
						*releaseDefinition.LastRelease.Id,
						*environment.Name,
						config.AutoApprove,
					)
					if err != nil {
						environmentLogger.Error(err)
					}
				} else {
					environmentLogger.Warn("unable to find latest release")
				}

			// redeploy current version
			case "current":
				if environment.CurrentRelease != nil && environment.CurrentRelease.Id != nil && *environment.CurrentRelease.Id != 0 {
					environmentLogger.Info("trigger current release")
					err := o.triggerExistingReleaseDeployment(
						environmentLogger,
						releaseDefinition,
						*environment.CurrentRelease.Id,
						*environment.Name,
						config.AutoApprove,
					)
					if err != nil {
						environmentLogger.Error(err)
					}
				} else {
					environmentLogger.Warn("unable to find current release")
				}
			}
		}
	}

	return nil
}

func (o *AzureDevopsOperator) triggerExistingReleaseDeployment(contextLogger *log.Entry, releaseDefinition *release.ReleaseDefinition, releaseId int, environmentName string, autoapprove bool) error {
	project := releaseDefinition.ProjectReference.Id.String()

	getArgs := release.GetReleaseArgs{
		Project:   &project,
		ReleaseId: &releaseId,
	}

	releaseRes, err := o.releaseClient.GetRelease(o.ctx, getArgs)
	if err != nil {
		return err
	}

	for _, environment := range *releaseRes.Environments {
		if *environment.Name == environmentName {
			if *environment.Status == release.EnvironmentStatusValues.Queued || *environment.Status == release.EnvironmentStatusValues.InProgress {
				_, err := o.updateReleaseEnvironment(contextLogger, releaseDefinition, environment, releaseTriggerCancelComment, release.EnvironmentStatusValues.Canceled, false)
				if err != nil {
					return err
				}
			}

			time.Sleep(timeWaitDeploy)

			_, err := o.updateReleaseEnvironment(contextLogger, releaseDefinition, environment, releaseTriggerNewComment, release.EnvironmentStatusValues.InProgress, autoapprove)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (o *AzureDevopsOperator) updateReleaseEnvironment(contextLogger *log.Entry, releaseDefinition *release.ReleaseDefinition, releaseEnvironment release.ReleaseEnvironment, comment string, status release.EnvironmentStatus, autoapprove bool) (*release.ReleaseEnvironment, error) {
	project := releaseDefinition.ProjectReference.Id.String()

	contextLogger.Infof("update release environment deployment of [%s]%s/%s: status -> %s", *releaseDefinition.ProjectReference.Name, *releaseEnvironment.Release.Name, *releaseEnvironment.Name, status)
	updateArgs := release.UpdateReleaseEnvironmentArgs{
		Project:       &project,
		ReleaseId:     releaseEnvironment.ReleaseId,
		EnvironmentId: releaseEnvironment.Id,
		EnvironmentUpdateData: &release.ReleaseEnvironmentUpdateMetadata{
			Comment:                 &comment,
			ScheduledDeploymentTime: nil,
			Status:                  &status,
		},
	}
	releaseRes, err := o.releaseClient.UpdateReleaseEnvironment(o.ctx, updateArgs)
	if err != nil {
		return releaseRes, err
	}

	// send notification if (re)deployment is triggered
	if status == release.EnvironmentStatusValues.InProgress {
		promLabels := prometheus.Labels{
			"projectId":             releaseDefinition.ProjectReference.Id.String(),
			"projectName":           *releaseDefinition.ProjectReference.Name,
			"releaseDefinitionName": o.buildReleaseDefinitionName(releaseDefinition),
			"environmentName":       *releaseEnvironment.Name,
		}

		o.prometheus.deploymentCounter.With(promLabels).Inc()
		o.prometheus.deploymentTime.With(promLabels).SetToCurrentTime()
		o.prometheus.deploymentStatus.With(promLabels).Set(1)

		msg := fmt.Sprintf(
			"starting deployment of [%s]%s :: %s -> %s",
			*releaseDefinition.ProjectReference.Name,
			o.buildReleaseDefinitionName(releaseDefinition),
			*releaseEnvironment.Release.Name,
			*releaseEnvironment.Name,
		)
		o.sendNotification(msg)
	}

	// check for approval
	if autoapprove {
		// wait to settle update, then get the new release to check if approval is needed
		time.Sleep(timeWaitDeploy)

		getArgs := release.GetReleaseEnvironmentArgs{
			Project:       &project,
			ReleaseId:     releaseEnvironment.ReleaseId,
			EnvironmentId: releaseEnvironment.Id,
		}
		releaseRes, err = o.releaseClient.GetReleaseEnvironment(o.ctx, getArgs)
		if err != nil {
			return releaseRes, err
		}

		// check if there are any predeployment approvals needed
		if releaseRes.PreDeployApprovals != nil {
			for _, approval := range *releaseRes.PreDeployApprovals {
				// check if approval is pending
				if approval.Status != nil && *approval.Status == release.ApprovalStatusValues.Pending {
					// approve the approval
					contextLogger.Infof("auto-approve release environment deployment of %s/%s", *releaseEnvironment.Release.Name, *releaseEnvironment.Name)
					approvalArgs := release.UpdateReleaseApprovalArgs{
						Project:    &project,
						ApprovalId: approval.Id,
						Approval: &release.ReleaseApproval{
							Status:   &release.ApprovalStatusValues.Approved,
							Comments: &releaseAutoApprovalComment,
						},
					}
					_, err := o.releaseClient.UpdateReleaseApproval(o.ctx, approvalArgs)
					if err != nil {
						return releaseRes, err
					}
				}
			}
		}
	}

	return releaseRes, nil
}

func (o *AzureDevopsOperator) buildReleaseDefinitionName(releaseDefinition *release.ReleaseDefinition) (name string) {
	// make releasename human readable
	name = *releaseDefinition.Name
	if releaseDefinition.Path != nil && *releaseDefinition.Path != "" {
		releasePath := *releaseDefinition.Path
		releasePath = strings.ReplaceAll(releasePath, "\\", "/")
		releasePath = strings.TrimRight(releasePath, "/")
		name = fmt.Sprintf("%s/%s", releasePath, name)
	}

	return
}

func (o *AzureDevopsOperator) sendNotification(message string, args ...interface{}) {
	message = fmt.Sprintf(message, args...)
	message = fmt.Sprintf(o.Notification.Template, message)

	for _, url := range o.Notification.Urls {
		if err := shoutrrr.Send(url, message); err != nil {
			log.Errorf("unable to send shoutrrr notification: %v", err)
		}
	}
}
