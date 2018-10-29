/*
Copyright 2017 The Kubernetes Authors.

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

package etcd

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	etcdutil "k8s.io/kubernetes/cmd/kubeadm/app/util/etcd"
	testutil "k8s.io/kubernetes/cmd/kubeadm/test"
)

func TestGetEtcdPodSpec(t *testing.T) {
	// Creates a Master Configuration
	cfg := &kubeadmapi.InitConfiguration{
		ClusterConfiguration: kubeadmapi.ClusterConfiguration{
			KubernetesVersion: "v1.7.0",
			Etcd: kubeadmapi.Etcd{
				Local: &kubeadmapi.LocalEtcd{
					DataDir: "/var/lib/etcd",
					Image:   "",
				},
			},
		},
	}

	// Executes GetEtcdPodSpec
	spec := GetEtcdPodSpec(cfg, []etcdutil.Member{})

	// Assert each specs refers to the right pod
	if spec.Spec.Containers[0].Name != kubeadmconstants.Etcd {
		t.Errorf("getKubeConfigSpecs spec for etcd contains pod %s, expects %s", spec.Spec.Containers[0].Name, kubeadmconstants.Etcd)
	}
}

func TestCreateLocalEtcdStaticPodManifestFile(t *testing.T) {
	// Create temp folder for the test case
	tmpdir := testutil.SetupTempDir(t)
	defer os.RemoveAll(tmpdir)

	var tests = []struct {
		cfg           *kubeadmapi.InitConfiguration
		expectedError bool
	}{
		{
			cfg: &kubeadmapi.InitConfiguration{
				ClusterConfiguration: kubeadmapi.ClusterConfiguration{
					KubernetesVersion: "v1.7.0",
					Etcd: kubeadmapi.Etcd{
						Local: &kubeadmapi.LocalEtcd{
							DataDir: "/var/lib/etcd",
							Image:   "k8s.gcr.io/etcd",
						},
					},
				},
			},
			expectedError: false,
		},
		{
			cfg: &kubeadmapi.InitConfiguration{
				ClusterConfiguration: kubeadmapi.ClusterConfiguration{
					KubernetesVersion: "v1.7.0",
					Etcd: kubeadmapi.Etcd{
						External: &kubeadmapi.ExternalEtcd{
							Endpoints: []string{
								"https://etcd-instance:2379",
							},
							CAFile:   "/etc/kubernetes/pki/etcd/ca.crt",
							CertFile: "/etc/kubernetes/pki/etcd/apiserver-etcd-client.crt",
							KeyFile:  "/etc/kubernetes/pki/etcd/apiserver-etcd-client.key",
						},
					},
				},
			},
			expectedError: true,
		},
	}

	for _, test := range tests {
		// Execute createStaticPodFunction
		manifestPath := filepath.Join(tmpdir, kubeadmconstants.ManifestsSubDirName)
		err := CreateLocalEtcdStaticPodManifestFile(manifestPath, test.cfg)

		if !test.expectedError {
			if err != nil {
				t.Errorf("CreateLocalEtcdStaticPodManifestFile failed when not expected: %v", err)
			}
			// Assert expected files are there
			testutil.AssertFilesCount(t, manifestPath, 1)
			testutil.AssertFileExists(t, manifestPath, kubeadmconstants.Etcd+".yaml")
		} else {
			testutil.AssertError(t, err, "etcd static pod manifest cannot be generated for cluster using external etcd")
		}
	}
}

func TestGetEtcdCommand(t *testing.T) {
	var tests = []struct {
		name           string
		cfg            *kubeadmapi.InitConfiguration
		initialCluster []etcdutil.Member
		expected       []string
	}{
		{
			name: "Default args - with empty etcd initial cluster",
			cfg: &kubeadmapi.InitConfiguration{
				APIEndpoint: kubeadmapi.APIEndpoint{
					AdvertiseAddress: "1.2.3.4",
				},
				NodeRegistration: kubeadmapi.NodeRegistrationOptions{
					Name: "foo",
				},
				ClusterConfiguration: kubeadmapi.ClusterConfiguration{
					Etcd: kubeadmapi.Etcd{
						Local: &kubeadmapi.LocalEtcd{
							DataDir: "/var/lib/etcd",
						},
					},
				},
			},
			expected: []string{
				"etcd",
				"--name=foo",
				fmt.Sprintf("--listen-client-urls=https://127.0.0.1:%d,https://1.2.3.4:%d", kubeadmconstants.EtcdListenClientPort, kubeadmconstants.EtcdListenClientPort),
				fmt.Sprintf("--advertise-client-urls=https://1.2.3.4:%d", kubeadmconstants.EtcdListenClientPort),
				fmt.Sprintf("--listen-peer-urls=https://1.2.3.4:%d", kubeadmconstants.EtcdListenPeerPort),
				fmt.Sprintf("--initial-advertise-peer-urls=https://1.2.3.4:%d", kubeadmconstants.EtcdListenPeerPort),
				"--data-dir=/var/lib/etcd",
				"--cert-file=" + kubeadmconstants.EtcdServerCertName,
				"--key-file=" + kubeadmconstants.EtcdServerKeyName,
				"--trusted-ca-file=" + kubeadmconstants.EtcdCACertName,
				"--client-cert-auth=true",
				"--peer-cert-file=" + kubeadmconstants.EtcdPeerCertName,
				"--peer-key-file=" + kubeadmconstants.EtcdPeerKeyName,
				"--peer-trusted-ca-file=" + kubeadmconstants.EtcdCACertName,
				"--snapshot-count=10000",
				"--peer-client-cert-auth=true",
				fmt.Sprintf("--initial-cluster=foo=https://1.2.3.4:%d", kubeadmconstants.EtcdListenPeerPort),
			},
		},
		{
			name: "Default args - With an existing etcd cluster",
			cfg: &kubeadmapi.InitConfiguration{
				APIEndpoint: kubeadmapi.APIEndpoint{
					AdvertiseAddress: "1.2.3.4",
				},
				NodeRegistration: kubeadmapi.NodeRegistrationOptions{
					Name: "foo",
				},
				ClusterConfiguration: kubeadmapi.ClusterConfiguration{
					Etcd: kubeadmapi.Etcd{
						Local: &kubeadmapi.LocalEtcd{
							DataDir: "/var/lib/etcd",
						},
					},
				},
			},
			initialCluster: []etcdutil.Member{
				{Name: "foo", PeerURL: fmt.Sprintf("https://1.2.3.4:%d", kubeadmconstants.EtcdListenPeerPort)}, // NB. the joining etcd instance should be part of the initialCluster list
				{Name: "bar", PeerURL: fmt.Sprintf("https://5.6.7.8:%d", kubeadmconstants.EtcdListenPeerPort)},
			},
			expected: []string{
				"etcd",
				"--name=foo",
				fmt.Sprintf("--listen-client-urls=https://127.0.0.1:%d,https://1.2.3.4:%d", kubeadmconstants.EtcdListenClientPort, kubeadmconstants.EtcdListenClientPort),
				fmt.Sprintf("--advertise-client-urls=https://1.2.3.4:%d", kubeadmconstants.EtcdListenClientPort),
				fmt.Sprintf("--listen-peer-urls=https://1.2.3.4:%d", kubeadmconstants.EtcdListenPeerPort),
				fmt.Sprintf("--initial-advertise-peer-urls=https://1.2.3.4:%d", kubeadmconstants.EtcdListenPeerPort),
				"--data-dir=/var/lib/etcd",
				"--cert-file=" + kubeadmconstants.EtcdServerCertName,
				"--key-file=" + kubeadmconstants.EtcdServerKeyName,
				"--trusted-ca-file=" + kubeadmconstants.EtcdCACertName,
				"--client-cert-auth=true",
				"--peer-cert-file=" + kubeadmconstants.EtcdPeerCertName,
				"--peer-key-file=" + kubeadmconstants.EtcdPeerKeyName,
				"--peer-trusted-ca-file=" + kubeadmconstants.EtcdCACertName,
				"--snapshot-count=10000",
				"--peer-client-cert-auth=true",
				"--initial-cluster-state=existing",
				fmt.Sprintf("--initial-cluster=foo=https://1.2.3.4:%d,bar=https://5.6.7.8:%d", kubeadmconstants.EtcdListenPeerPort, kubeadmconstants.EtcdListenPeerPort),
			},
		},
		{
			name: "Extra args",
			cfg: &kubeadmapi.InitConfiguration{
				APIEndpoint: kubeadmapi.APIEndpoint{
					AdvertiseAddress: "1.2.3.4",
				},
				NodeRegistration: kubeadmapi.NodeRegistrationOptions{
					Name: "bar",
				},
				ClusterConfiguration: kubeadmapi.ClusterConfiguration{
					Etcd: kubeadmapi.Etcd{
						Local: &kubeadmapi.LocalEtcd{
							DataDir: "/var/lib/etcd",
							ExtraArgs: map[string]string{
								"listen-client-urls":    "https://10.0.1.10:2379",
								"advertise-client-urls": "https://10.0.1.10:2379",
							},
						},
					},
				},
			},
			expected: []string{
				"etcd",
				"--name=bar",
				"--listen-client-urls=https://10.0.1.10:2379",
				"--advertise-client-urls=https://10.0.1.10:2379",
				fmt.Sprintf("--listen-peer-urls=https://1.2.3.4:%d", kubeadmconstants.EtcdListenPeerPort),
				fmt.Sprintf("--initial-advertise-peer-urls=https://1.2.3.4:%d", kubeadmconstants.EtcdListenPeerPort),
				"--data-dir=/var/lib/etcd",
				"--cert-file=" + kubeadmconstants.EtcdServerCertName,
				"--key-file=" + kubeadmconstants.EtcdServerKeyName,
				"--trusted-ca-file=" + kubeadmconstants.EtcdCACertName,
				"--client-cert-auth=true",
				"--peer-cert-file=" + kubeadmconstants.EtcdPeerCertName,
				"--peer-key-file=" + kubeadmconstants.EtcdPeerKeyName,
				"--peer-trusted-ca-file=" + kubeadmconstants.EtcdCACertName,
				"--snapshot-count=10000",
				"--peer-client-cert-auth=true",
				fmt.Sprintf("--initial-cluster=bar=https://1.2.3.4:%d", kubeadmconstants.EtcdListenPeerPort),
			},
		},
	}

	for _, rt := range tests {
		t.Run(rt.name, func(t *testing.T) {
			actual := getEtcdCommand(rt.cfg, rt.initialCluster)
			sort.Strings(actual)
			sort.Strings(rt.expected)
			if !reflect.DeepEqual(actual, rt.expected) {
				t.Errorf("failed getEtcdCommand:\nexpected:\n%v\nsaw:\n%v", rt.expected, actual)
			}
		})
	}
}