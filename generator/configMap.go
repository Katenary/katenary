package generator

import (
	"fmt"
	"katenary/generator/labels"
	"katenary/generator/labels/labelstructs"
	"katenary/utils"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/compose-spec/compose-go/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FileMapUsage is the usage of the filemap.
type FileMapUsage uint8

// FileMapUsage constants.
const (
	FileMapUsageConfigMap FileMapUsage = iota // pure configmap for key:values.
	FileMapUsageFiles                         // files in a configmap.
)

// only used to check interface implementation
var (
	_ DataMap = (*ConfigMap)(nil)
	_ Yaml    = (*ConfigMap)(nil)
)

// ConfigMap is a kubernetes ConfigMap.
// Implements the DataMap interface.
type ConfigMap struct {
	*corev1.ConfigMap
	service *types.ServiceConfig
	path    string
	usage   FileMapUsage
}

// NewConfigMap creates a new ConfigMap from a compose service. The appName is the name of the application taken from the project name.
// The ConfigMap is filled by environment variables and labels "map-env".
func NewConfigMap(service types.ServiceConfig, appName string, forFile bool) *ConfigMap {
	done := map[string]bool{}
	drop := map[string]bool{}
	labelValues := []string{}

	cm := &ConfigMap{
		service: &service,
		ConfigMap: &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:        utils.TplName(service.Name, appName),
				Labels:      GetLabels(service.Name, appName),
				Annotations: Annotations,
			},
			Data: make(map[string]string),
		},
	}

	// get the secrets from the labels
	secrets, err := labelstructs.SecretsFrom(service.Labels[labels.LabelSecrets])
	if err != nil {
		log.Fatal(err)
	}
	// drop the secrets from the environment
	for _, secret := range secrets {
		drop[secret] = true
	}
	// get the label values from the labels
	varDescriptons := utils.GetValuesFromLabel(service, labels.LabelValues)
	for value := range varDescriptons {
		labelValues = append(labelValues, value)
	}

	// change the environment variables to the values defined in the values.yaml
	for _, value := range labelValues {
		if _, ok := service.Environment[value]; !ok {
			done[value] = true
			continue
		}
	}

	if !forFile {
		// do not bind env variables to the configmap
		// remove the variables that are already defined in the environment
		if l, ok := service.Labels[labels.LabelMapEnv]; ok {
			envmap, err := labelstructs.MapEnvFrom(l)
			if err != nil {
				log.Fatal("Error parsing map-env", err)
			}
			for key, value := range envmap {
				cm.AddData(key, strings.ReplaceAll(value, "__APP__", appName))
				done[key] = true
			}
		}
		for key, env := range service.Environment {
			_, isDropped := drop[key]
			_, isDone := done[key]
			if isDropped || isDone {
				continue
			}
			cm.AddData(key, *env)
		}
	}

	return cm
}

// NewConfigMapFromDirectory creates a new ConfigMap from a compose service. This path is the path to the
// file or directory. If the path is a directory, all files in the directory are added to the ConfigMap.
// Each subdirectory are ignored. Note that the Generate() function will create the subdirectories ConfigMaps.
func NewConfigMapFromDirectory(service types.ServiceConfig, appName, path string) *ConfigMap {
	normalized := path
	normalized = strings.TrimLeft(normalized, ".")
	normalized = strings.TrimLeft(normalized, "/")
	normalized = regexp.MustCompile(`[^a-zA-Z0-9-]+`).ReplaceAllString(normalized, "-")

	cm := &ConfigMap{
		path:    path,
		service: &service,
		usage:   FileMapUsageFiles,
		ConfigMap: &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:        utils.TplName(service.Name, appName) + "-" + normalized,
				Labels:      GetLabels(service.Name, appName),
				Annotations: Annotations,
			},
			Data: make(map[string]string),
		},
	}
	// cumulate the path to the WorkingDir
	path = filepath.Join(service.WorkingDir, path)
	path = filepath.Clean(path)
	if err := cm.AppendDir(path); err != nil {
		log.Fatal("Error adding files to configmap:", err)
	}
	return cm
}

// AddData adds a key value pair to the configmap. Append or overwrite the value if the key already exists.
func (c *ConfigMap) AddData(key, value string) {
	c.Data[key] = value
}

// AddBinaryData adds binary data to the configmap. Append or overwrite the value if the key already exists.
func (c *ConfigMap) AddBinaryData(key string, value []byte) {
	if c.BinaryData == nil {
		c.BinaryData = make(map[string][]byte)
	}
	c.BinaryData[key] = value
}

// AppendDir adds files from given path to the configmap. It is not recursive, to add all files in a directory,
// you need to call this function for each subdirectory.
func (c *ConfigMap) AppendDir(path string) error {
	// read all files in the path and add them to the configmap
	stat, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("path %s does not exist, %w", path, err)
	}
	// recursively read all files in the path and add them to the configmap
	if stat.IsDir() {
		files, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		for _, file := range files {
			if file.IsDir() {
				utils.Warn("Subdirectories are ignored for the moment, skipping", filepath.Join(path, file.Name()))
				continue
			}
			path := filepath.Join(path, file.Name())
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			// remove the path from the file
			filename := filepath.Base(path)
			if utf8.Valid(content) {
				c.AddData(filename, string(content))
			} else {
				c.AddBinaryData(filename, content)
			}

		}
	} else {
		// add the file to the configmap
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		filename := filepath.Base(path)
		if utf8.Valid(content) {
			c.AddData(filename, string(content))
		} else {
			c.AddBinaryData(filename, content)
		}
	}
	return nil
}

func (c *ConfigMap) AppendFile(path string) error {
	// read all files in the path and add them to the configmap
	stat, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("path %s doesn not exists, %w", path, err)
	}
	// recursively read all files in the path and add them to the configmap
	if !stat.IsDir() {
		// add the file to the configmap
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if utf8.Valid(content) {
			c.AddData(filepath.Base(path), string(content))
		} else {
			c.AddBinaryData(filepath.Base(path), content)
		}

	}
	return nil
}

// Filename returns the filename of the configmap. If the configmap is used for files, the filename contains the path.
func (c *ConfigMap) Filename() string {
	switch c.usage {
	case FileMapUsageFiles:
		return filepath.Join(c.service.Name, "statics", c.path, "configmap.yaml")
	default:
		return c.service.Name + ".configmap.yaml"
	}
}

// SetData sets the data of the configmap. It replaces the entire data.
func (c *ConfigMap) SetData(data map[string]string) {
	c.Data = data
}

// Yaml returns the yaml representation of the configmap
func (c *ConfigMap) Yaml() ([]byte, error) {
	return ToK8SYaml(c)
}
