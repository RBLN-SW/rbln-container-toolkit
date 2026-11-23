/*
Copyright 2026 Rebellions Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cdi

import (
	"bytes"
	"encoding/json"
	"os"

	"gopkg.in/yaml.v3"
	"tags.cncf.io/container-device-interface/specs-go"
)

// parseSpec decodes a CDI spec file (YAML or JSON) the way container runtimes
// built on CDI v1.1.1 or later do: yaml.v3 with KnownFields(true) against the
// CDI key names. Keys are matched case-sensitively and unknown keys are
// rejected, so a key that CRI-O 1.37, containerd 2.4, Docker 29.10 or Podman
// 6.2 would refuse fails here too. specs-go v0.7.0 carries JSON tags only, so
// the spec is decoded into cdiFileSpec first and converted through JSON.
func parseSpec(data []byte) (*specs.Spec, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)

	var raw cdiFileSpec
	if err := dec.Decode(&raw); err != nil {
		return nil, err
	}

	b, err := json.Marshal(&raw)
	if err != nil {
		return nil, err
	}
	var spec specs.Spec
	if err := json.Unmarshal(b, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

// cdiFileSpec and the types below mirror the key names of the CDI spec as
// defined by tags.cncf.io/container-device-interface/specs-go v1.1.x. The
// yaml tags drive the strict decode in parseSpec; the json tags use the same
// key names and carry the result into specs.Spec. The YAML writer renders
// through the same types (see toCDIFileSpec), so reading and writing share one
// definition of every key. Yaml flow style only affects encoding: mount
// options render inline as `options: [ro, bind]` and decode either way.
type cdiFileSpec struct {
	Version        string                `yaml:"cdiVersion" json:"cdiVersion"`
	Kind           string                `yaml:"kind" json:"kind"`
	Annotations    map[string]string     `yaml:"annotations,omitempty" json:"annotations,omitempty"`
	Devices        []cdiFileDevice       `yaml:"devices" json:"devices"`
	ContainerEdits cdiFileContainerEdits `yaml:"containerEdits,omitempty" json:"containerEdits"`
}

type cdiFileDevice struct {
	Name           string                `yaml:"name" json:"name"`
	Annotations    map[string]string     `yaml:"annotations,omitempty" json:"annotations,omitempty"`
	ContainerEdits cdiFileContainerEdits `yaml:"containerEdits" json:"containerEdits"`
}

type cdiFileContainerEdits struct {
	Env            []string             `yaml:"env,omitempty" json:"env,omitempty"`
	DeviceNodes    []*cdiFileDeviceNode `yaml:"deviceNodes,omitempty" json:"deviceNodes,omitempty"`
	NetDevices     []*cdiFileNetDevice  `yaml:"netDevices,omitempty" json:"netDevices,omitempty"`
	Hooks          []*cdiFileHook       `yaml:"hooks,omitempty" json:"hooks,omitempty"`
	Mounts         []*cdiFileMount      `yaml:"mounts,omitempty" json:"mounts,omitempty"`
	IntelRdt       *cdiFileIntelRdt     `yaml:"intelRdt,omitempty" json:"intelRdt,omitempty"`
	AdditionalGIDs []uint32             `yaml:"additionalGids,omitempty" json:"additionalGids,omitempty"`
}

type cdiFileDeviceNode struct {
	Path        string       `yaml:"path" json:"path"`
	HostPath    string       `yaml:"hostPath,omitempty" json:"hostPath,omitempty"`
	Type        string       `yaml:"type,omitempty" json:"type,omitempty"`
	Major       int64        `yaml:"major,omitempty" json:"major,omitempty"`
	Minor       int64        `yaml:"minor,omitempty" json:"minor,omitempty"`
	FileMode    *os.FileMode `yaml:"fileMode,omitempty" json:"fileMode,omitempty"`
	Permissions string       `yaml:"permissions,omitempty" json:"permissions,omitempty"`
	UID         *uint32      `yaml:"uid,omitempty" json:"uid,omitempty"`
	GID         *uint32      `yaml:"gid,omitempty" json:"gid,omitempty"`
}

type cdiFileNetDevice struct {
	HostInterfaceName string `yaml:"hostInterfaceName" json:"hostInterfaceName"`
	Name              string `yaml:"name" json:"name"`
}

type cdiFileHook struct {
	HookName string   `yaml:"hookName" json:"hookName"`
	Path     string   `yaml:"path" json:"path"`
	Args     []string `yaml:"args,omitempty" json:"args,omitempty"`
	Env      []string `yaml:"env,omitempty" json:"env,omitempty"`
	Timeout  *int     `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

type cdiFileMount struct {
	HostPath      string   `yaml:"hostPath" json:"hostPath"`
	ContainerPath string   `yaml:"containerPath" json:"containerPath"`
	Options       []string `yaml:"options,omitempty,flow" json:"options,omitempty"`
	Type          string   `yaml:"type,omitempty" json:"type,omitempty"`
}

type cdiFileIntelRdt struct {
	ClosID           string   `yaml:"closID,omitempty" json:"closID,omitempty"`
	L3CacheSchema    string   `yaml:"l3CacheSchema,omitempty" json:"l3CacheSchema,omitempty"`
	MemBwSchema      string   `yaml:"memBwSchema,omitempty" json:"memBwSchema,omitempty"`
	Schemata         []string `yaml:"schemata,omitempty" json:"schemata,omitempty"`
	EnableMonitoring bool     `yaml:"enableMonitoring,omitempty" json:"enableMonitoring,omitempty"`
}
