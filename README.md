AzureDevOps Deployment operator
===============================

[![license](https://img.shields.io/github/license/webdevops/azuredevops-deployment-operator.svg)](https://github.com/webdevops/azuredevops-deployment-operator/blob/master/LICENSE)
[![DockerHub](https://img.shields.io/badge/DockerHub-webdevops%2Fazuredevops--deployment--operator-blue)](https://hub.docker.com/r/webdevops/azuredevops-deployment-operator/)
[![Quay.io](https://img.shields.io/badge/Quay.io-webdevops%2Fazuredevops--deployment--operator-blue)](https://quay.io/repository/webdevops/azuredevops-deployment-operator)

Triggers (re)deployments of releases in AzureDevOps and also auto approves them if configured.

Supports [shoutrrr](https://containrrr.github.io/shoutrrr/) notifications.

Configuration
-------------

```
Usage:
  azuredevops-deployment-operator [OPTIONS]

Application Options:
      --dry-run                      Dry run (no redeploy triggered) [$DRY_RUN]
      --config=                      Config file (yaml) [$CONFIG]
      --singleshot                   Trigger deployments and exit (ignoring cron) [$SINGLESHOT]
      --debug                        debug mode [$DEBUG]
  -v, --verbose                      verbose mode [$VERBOSE]
      --log.json                     Switch log output to json format [$LOG_JSON]
      --azuredevops.organizationurl= Url to AzureDevops organization (eg. https://dev.azure.com/myorg) [$AZUREDEVOPS_ORGANIZATIONURL]
      --azuredevops.accesstoken=     Personal access token [$AZUREDEVOPS_ACCESSTOKEN]
      --notification.template=       Notification template (default: %v) [$NOTIFICATION_TEMPLATE]
      --notification=                Shoutrrr url for notifications (https://containrrr.github.io/shoutrrr/) [$NOTIFICATION]
      --bind=                        Server address (default: :8080) [$SERVER_BIND]

Help Options:
  -h, --help                         Show this help message
```

for configuration file see [`example.yaml`](/example.yaml)

Metrics
-------

 (see `:8080/metrics`)

| Metric                                                   | Description                                     |
|:---------------------------------------------------------|:------------------------------------------------|
| `azuredevops_deployment_operator_deployment_counter`     | Count of (re)deployments                        |
| `azuredevops_deployment_operator_deployment_status`      | Status if deployment was triggered              |
| `azuredevops_deployment_operator_deployment_time`        | Last deployment time                            |
