package config

type (
	Config struct {
		Releases []ConfigRelease `yaml:"releases"`
	}

	ConfigRelease struct {
		Crontab      string   `yaml:"crontab"`
		Project      *string  `yaml:"project"`
		Environments []string `yaml:"environments"`
		Trigger      string   `yaml:"trigger"`
		AutoApprove  bool     `yaml:"autoApprove"`

		ReleaseDefinitions struct {
			SearchText                   *string   `yaml:"searchText"`
			ArtifactType                 *string   `yaml:"artifactType"`
			ArtifactSourceId             *string   `yaml:"artifactSourceId"`
			Path                         *string   `yaml:"path"`
			IsExactNameMatch             *bool     `yaml:"isExactNameMatch"`
			TagFilter                    *[]string `yaml:"tagFilter"`
			DefinitionIdFilter           *[]string `yaml:"definitionIdFilter"`
			SearchTextContainsFolderName *bool     `yaml:"searchTextContainsFolderName"`
		} `yaml:"releaseDefinitions"`
	}
)
