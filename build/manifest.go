package build

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"github.com/cloud66/habitus/configuration"

	"gopkg.in/yaml.v2"
)

// Artefact holds a parsed source for a build artefact
type Artefact struct {
	Order  int
	Step   Step
	Source string
	Dest   string // this is only the folder. Filename comes from the source
}

// Cleanup holds everything that's needed for a cleanup
type Cleanup struct {
	Commands []string
}

// Step Holds a single step in the build process
// Public structs. They are used to store the build for the builders
type Step struct {
	Order      int
	Name       string
	Dockerfile string
	Artefacts  []Artefact
	Manifest   Manifest
	Cleanup    *Cleanup
}

// Manifest Holds the whole build process
type Manifest struct {
	Steps        []Step
	IsPrivileged bool
}

type cleanup struct {
	Commands []string
}

// Private structs. They are used to load from yaml
type step struct {
	Name       string
	Dockerfile string
	Artefacts  []string
	Cleanup    *cleanup
}

type build struct {
	Workdir string
	Steps   []step
	Config  *configuration.Config
}

// LoadBuildFromFile loads Build from a yaml file
func LoadBuildFromFile(config *configuration.Config) (*Manifest, error) {
	config.Logger.Notice("Using '%s' as build file", config.Buildfile)

	t := build{Config: config}

	data, err := ioutil.ReadFile(config.Buildfile)
	if err != nil {
		return nil, err
	}

	data = parseForEnvVars(config, data)

	err = yaml.Unmarshal([]byte(data), &t)
	if err != nil {
		return nil, err
	}

	return t.convertToBuild()
}

func (b *build) convertToBuild() (*Manifest, error) {
	r := Manifest{}
	r.IsPrivileged = false
	r.Steps = []Step{}

	for idx, s := range b.Steps {
		convertedStep := Step{}

		convertedStep.Manifest = r
		convertedStep.Dockerfile = s.Dockerfile
		convertedStep.Name = s.Name
		convertedStep.Order = idx
		convertedStep.Artefacts = []Artefact{}
		if s.Cleanup != nil && !b.Config.NoSquash {
			convertedStep.Cleanup = &Cleanup{Commands: s.Cleanup.Commands}
			r.IsPrivileged = true
		} else {
			convertedStep.Cleanup = &Cleanup{}
		}

		for kdx, a := range s.Artefacts {
			convertedArt := Artefact{}

			convertedArt.Order = kdx
			convertedArt.Step = convertedStep
			parts := strings.Split(a, ":")
			convertedArt.Source = parts[0]
			if len(parts) == 1 {
				// only one use the base
				convertedArt.Dest = "."
			} else {
				convertedArt.Dest = parts[1]
			}

			convertedStep.Artefacts = append(convertedStep.Artefacts, convertedArt)
		}

		// is it unique?
		for _, s := range r.Steps {
			if s.Name == convertedStep.Name {
				return nil, fmt.Errorf("Step name '%s' is not unique", convertedStep.Name)
			}
		}

		r.Steps = append(r.Steps, convertedStep)
	}

	return &r, nil
}

// FindStepByName finds a step by name. Returns nil if not found
func (m *Manifest) FindStepByName(name string) (*Step, error) {
	for _, step := range m.Steps {
		if step.Name == name {
			return &step, nil
		}
	}

	return nil, nil
}

func parseForEnvVars(config *configuration.Config, value []byte) []byte {
	r, _ := regexp.Compile("_env\\((.*)\\)")

	matched := r.ReplaceAllFunc(value, func(s []byte) []byte {
		m := string(s)
		parts := r.FindStringSubmatch(m)

		if len(config.EnvVars) == 0 {
			return []byte(os.Getenv(parts[1]))
		} else {
			return []byte(config.EnvVars.Find(parts[1]))
		}
	})

	return matched
}