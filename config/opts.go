package config

type (
	Opts struct {
		// general settings
		DryRun     bool   `long:"dry-run"       env:"DRY_RUN"   description:"Dry run (no redeploy triggered)"`
		ConfigPath string `long:"config"        env:"CONFIG"   description:"Config file (yaml)" required:"true"`

		// logger
		Logger struct {
			Debug   bool `           long:"debug"        env:"DEBUG"    description:"debug mode"`
			Verbose bool `short:"v"  long:"verbose"      env:"VERBOSE"  description:"verbose mode"`
			LogJson bool `           long:"log.json"     env:"LOG_JSON" description:"Switch log output to json format"`
		}

		// azuredevops
		AzureDevops OptsAzureDevops

		// notification
		Notification OptsNotification

		// server settings
		ServerBind string `long:"bind" env:"SERVER_BIND"  description:"Server address"  default:":8080"`
	}

	OptsAzureDevops struct {
		OrganizationUrl string `long:"azuredevops.organizationurl"        env:"AZUREDEVOPS_ORGANIZATIONURL"    description:"Url to AzureDevops organization (eg. https://dev.azure.com/myorg)" required:"true"`
		AccessToken     string `long:"azuredevops.accesstoken"        env:"AZUREDEVOPS_ACCESSTOKEN"    description:"Personal access token"  required:"true"`
	}

	OptsNotification struct {
		Template string   `long:"notification.template" env:"NOTIFICATION_TEMPLATE"  description:"Notification template" default:"%v"`
		Urls     []string `long:"notification" env:"NOTIFICATION" description:"Shoutrrr url for notifications (https://containrrr.github.io/shoutrrr/)" env-delim:" "`
	}
)
