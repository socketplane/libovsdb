package client

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/cenkalti/rpc2"
	"github.com/ovn-org/libovsdb/cache"
	"github.com/ovn-org/libovsdb/model"
	"github.com/ovn-org/libovsdb/ovsdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	aUUID0 = "2f77b348-9768-4866-b761-89d5177ecda0"
	aUUID1 = "2f77b348-9768-4866-b761-89d5177ecda1"
	aUUID2 = "2f77b348-9768-4866-b761-89d5177ecda2"
	aUUID3 = "2f77b348-9768-4866-b761-89d5177ecda3"
)

type (
	BridgeFailMode  = string
	BridgeProtocols = string
)

const (
	BridgeFailModeStandalone  BridgeFailMode  = "standalone"
	BridgeFailModeSecure      BridgeFailMode  = "secure"
	BridgeProtocolsOpenflow10 BridgeProtocols = "OpenFlow10"
	BridgeProtocolsOpenflow11 BridgeProtocols = "OpenFlow11"
	BridgeProtocolsOpenflow12 BridgeProtocols = "OpenFlow12"
	BridgeProtocolsOpenflow13 BridgeProtocols = "OpenFlow13"
	BridgeProtocolsOpenflow14 BridgeProtocols = "OpenFlow14"
	BridgeProtocolsOpenflow15 BridgeProtocols = "OpenFlow15"
)

// Bridge defines an object in Bridge table
type Bridge struct {
	UUID                string            `ovsdb:"_uuid"`
	AutoAttach          []string          `ovsdb:"auto_attach"`
	Controller          []string          `ovsdb:"controller"`
	DatapathID          []string          `ovsdb:"datapath_id"`
	DatapathType        string            `ovsdb:"datapath_type"`
	DatapathVersion     string            `ovsdb:"datapath_version"`
	ExternalIDs         map[string]string `ovsdb:"external_ids"`
	FailMode            []BridgeFailMode  `ovsdb:"fail_mode"`
	FloodVLANs          []int             `ovsdb:"flood_vlans"`
	FlowTables          map[int]string    `ovsdb:"flow_tables"`
	IPFIX               []string          `ovsdb:"ipfix"`
	McastSnoopingEnable bool              `ovsdb:"mcast_snooping_enable"`
	Mirrors             []string          `ovsdb:"mirrors"`
	Name                string            `ovsdb:"name"`
	Netflow             []string          `ovsdb:"netflow"`
	OtherConfig         map[string]string `ovsdb:"other_config"`
	Ports               []string          `ovsdb:"ports"`
	Protocols           []BridgeProtocols `ovsdb:"protocols"`
	RSTPEnable          bool              `ovsdb:"rstp_enable"`
	RSTPStatus          map[string]string `ovsdb:"rstp_status"`
	Sflow               []string          `ovsdb:"sflow"`
	Status              map[string]string `ovsdb:"status"`
	STPEnable           bool              `ovsdb:"stp_enable"`
}

// OpenvSwitch defines an object in Open_vSwitch table
type OpenvSwitch struct {
	UUID            string            `ovsdb:"_uuid"`
	Bridges         []string          `ovsdb:"bridges"`
	CurCfg          int               `ovsdb:"cur_cfg"`
	DatapathTypes   []string          `ovsdb:"datapath_types"`
	Datapaths       map[string]string `ovsdb:"datapaths"`
	DbVersion       []string          `ovsdb:"db_version"`
	DpdkInitialized bool              `ovsdb:"dpdk_initialized"`
	DpdkVersion     []string          `ovsdb:"dpdk_version"`
	ExternalIDs     map[string]string `ovsdb:"external_ids"`
	IfaceTypes      []string          `ovsdb:"iface_types"`
	ManagerOptions  []string          `ovsdb:"manager_options"`
	NextCfg         int               `ovsdb:"next_cfg"`
	OtherConfig     map[string]string `ovsdb:"other_config"`
	OVSVersion      []string          `ovsdb:"ovs_version"`
	SSL             []string          `ovsdb:"ssl"`
	Statistics      map[string]string `ovsdb:"statistics"`
	SystemType      []string          `ovsdb:"system_type"`
	SystemVersion   []string          `ovsdb:"system_version"`
}

var defDB, _ = model.NewDBModel("Open_vSwitch",
	map[string]model.Model{
		"Open_vSwitch": &OpenvSwitch{},
		"Bridge":       &Bridge{},
	},
)

var schema = `{
	"name": "Open_vSwitch",
	"version": "8.2.0",
	"tables": {
	  "Bridge": {
		"columns": {
		  "auto_attach": {
			"type": {
			  "key": {
				"type": "uuid",
				"refTable": "AutoAttach"
			  },
			  "min": 0,
			  "max": 1
			}
		  },
		  "controller": {
			"type": {
			  "key": {
				"type": "uuid",
				"refTable": "Controller"
			  },
			  "min": 0,
			  "max": "unlimited"
			}
		  },
		  "datapath_id": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "min": 0,
			  "max": 1
			},
			"ephemeral": true
		  },
		  "datapath_type": {
			"type": "string"
		  },
		  "datapath_version": {
			"type": "string"
		  },
		  "external_ids": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "value": {
				"type": "string"
			  },
			  "min": 0,
			  "max": "unlimited"
			}
		  },
		  "fail_mode": {
			"type": {
			  "key": {
				"type": "string",
				"enum": [
				  "set",
				  [
					"standalone",
					"secure"
				  ]
				]
			  },
			  "min": 0,
			  "max": 1
			}
		  },
		  "flood_vlans": {
			"type": {
			  "key": {
				"type": "integer",
				"minInteger": 0,
				"maxInteger": 4095
			  },
			  "min": 0,
			  "max": 4096
			}
		  },
		  "flow_tables": {
			"type": {
			  "key": {
				"type": "integer",
				"minInteger": 0,
				"maxInteger": 254
			  },
			  "value": {
				"type": "uuid",
				"refTable": "Flow_Table"
			  },
			  "min": 0,
			  "max": "unlimited"
			}
		  },
		  "ipfix": {
			"type": {
			  "key": {
				"type": "uuid",
				"refTable": "IPFIX"
			  },
			  "min": 0,
			  "max": 1
			}
		  },
		  "mcast_snooping_enable": {
			"type": "boolean"
		  },
		  "mirrors": {
			"type": {
			  "key": {
				"type": "uuid",
				"refTable": "Mirror"
			  },
			  "min": 0,
			  "max": "unlimited"
			}
		  },
		  "name": {
			"type": "string",
			"mutable": false
		  },
		  "netflow": {
			"type": {
			  "key": {
				"type": "uuid",
				"refTable": "NetFlow"
			  },
			  "min": 0,
			  "max": 1
			}
		  },
		  "other_config": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "value": {
				"type": "string"
			  },
			  "min": 0,
			  "max": "unlimited"
			}
		  },
		  "ports": {
			"type": {
			  "key": {
				"type": "uuid",
				"refTable": "Port"
			  },
			  "min": 0,
			  "max": "unlimited"
			}
		  },
		  "protocols": {
			"type": {
			  "key": {
				"type": "string",
				"enum": [
				  "set",
				  [
					"OpenFlow10",
					"OpenFlow11",
					"OpenFlow12",
					"OpenFlow13",
					"OpenFlow14",
					"OpenFlow15"
				  ]
				]
			  },
			  "min": 0,
			  "max": "unlimited"
			}
		  },
		  "rstp_enable": {
			"type": "boolean"
		  },
		  "rstp_status": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "value": {
				"type": "string"
			  },
			  "min": 0,
			  "max": "unlimited"
			},
			"ephemeral": true
		  },
		  "sflow": {
			"type": {
			  "key": {
				"type": "uuid",
				"refTable": "sFlow"
			  },
			  "min": 0,
			  "max": 1
			}
		  },
		  "status": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "value": {
				"type": "string"
			  },
			  "min": 0,
			  "max": "unlimited"
			},
			"ephemeral": true
		  },
		  "stp_enable": {
			"type": "boolean"
		  }
		},
		"indexes": [
		  [
			"name"
		  ]
		]
	  },
	  "Open_vSwitch": {
		"columns": {
		  "bridges": {
			"type": {
			  "key": {
				"type": "uuid",
				"refTable": "Bridge"
			  },
			  "min": 0,
			  "max": "unlimited"
			}
		  },
		  "cur_cfg": {
			"type": "integer"
		  },
		  "datapath_types": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "min": 0,
			  "max": "unlimited"
			}
		  },
		  "datapaths": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "value": {
				"type": "uuid",
				"refTable": "Datapath"
			  },
			  "min": 0,
			  "max": "unlimited"
			}
		  },
		  "db_version": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "min": 0,
			  "max": 1
			}
		  },
		  "dpdk_initialized": {
			"type": "boolean"
		  },
		  "dpdk_version": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "min": 0,
			  "max": 1
			}
		  },
		  "external_ids": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "value": {
				"type": "string"
			  },
			  "min": 0,
			  "max": "unlimited"
			}
		  },
		  "iface_types": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "min": 0,
			  "max": "unlimited"
			}
		  },
		  "manager_options": {
			"type": {
			  "key": {
				"type": "uuid",
				"refTable": "Manager"
			  },
			  "min": 0,
			  "max": "unlimited"
			}
		  },
		  "next_cfg": {
			"type": "integer"
		  },
		  "other_config": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "value": {
				"type": "string"
			  },
			  "min": 0,
			  "max": "unlimited"
			}
		  },
		  "ovs_version": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "min": 0,
			  "max": 1
			}
		  },
		  "ssl": {
			"type": {
			  "key": {
				"type": "uuid",
				"refTable": "SSL"
			  },
			  "min": 0,
			  "max": 1
			}
		  },
		  "statistics": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "value": {
				"type": "string"
			  },
			  "min": 0,
			  "max": "unlimited"
			},
			"ephemeral": true
		  },
		  "system_type": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "min": 0,
			  "max": 1
			}
		  },
		  "system_version": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "min": 0,
			  "max": 1
			}
		  }
		}
	  }
	}
  }`

func testOvsSet(t *testing.T, set interface{}) ovsdb.OvsSet {
	oSet, err := ovsdb.NewOvsSet(set)
	assert.Nil(t, err)
	return oSet
}

func testOvsMap(t *testing.T, set interface{}) ovsdb.OvsMap {
	oMap, err := ovsdb.NewOvsMap(set)
	assert.Nil(t, err)
	return oMap
}

func updateBenchmark(ovs *ovsdbClient, updates []byte, b *testing.B) {
	for n := 0; n < b.N; n++ {
		params := []json.RawMessage{[]byte(`"v1"`), updates}
		var reply []interface{}
		err := ovs.update(params, &reply)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func newBridgeRow(name string) string {
	return `{
		"connection_mode": [ "set", [] ],
		"controller": [ "set", [] ],
		"datapath_id": "blablabla",
		"datapath_type": "",
		"datapath_version": "",
		"external_ids": [ "map", [["foo","bar"]]],
		"fail_mode": [ "set", [] ],
		"flood_vlans": [ "set", [] ],
		"flow_tables": [ "map", [] ],
		"ipfix": [ "set", [] ],
		"mcast_snooping_enable": false,
		"mirrors": [ "set", [] ],
		"name": "` + name + `",
		"netflow": [ "set", [] ],
		"other_config": [ "map", [["bar","quux"]]],
		"ports": [ "set", [] ],
		"protocols": [ "set", [] ],
		"rstp_enable": false,
		"rstp_status": [ "map", [] ],
		"sflow": [ "set", [] ],
		"status": [ "map", [] ],
		"stp_enable": false
	}`
}

func newOvsRow(bridges ...string) string {
	bridgeUUIDs := []string{}
	for _, b := range bridges {
		bridgeUUIDs = append(bridgeUUIDs, `[ "uuid", "`+b+`" ]`)
	}
	return `{
		"bridges": [ "set", [` + strings.Join(bridgeUUIDs, `,`) + `]],
		"cur_cfg": 0,
		"datapath_types": [ "set", [] ],
		"datapaths": [ "map", [] ],
		"db_version":       "8.2.0",
		"dpdk_initialized": false,
		"dpdk_version":     [ "set", [] ],
		"external_ids":     [ "map", [["system-id","829f8534-94a8-468e-9176-132738cf260a"]]],
		"iface_types":      [ "set", [] ],
		"manager_options":  ["uuid", "6e4cd5fc-f51a-462a-b3d6-a696af6d7a84"],
		"next_cfg":         0,
		"other_config":     [ "map", [] ],
		"ovs_version":      "2.15.90",
		"ssl":              [ "set", [] ],
		"statistics":       [ "map", [] ],
		"system_type":      "docker-ovs",
		"system_version":   "0.1"
	}`
}

func BenchmarkUpdate1(b *testing.B) {
	ovs, err := newOVSDBClient(defDB)
	require.NoError(b, err)
	var s ovsdb.DatabaseSchema
	err = json.Unmarshal([]byte(schema), &s)
	require.NoError(b, err)
	dbModel, err := model.NewDBModel("Open_vSwitch", map[string]model.Model{
		"Bridge":       &Bridge{},
		"Open_vSwitch": &OpenvSwitch{},
	})
	require.NoError(b, err)
	ovs.cache, err = cache.NewTableCache(&s, dbModel, nil)
	require.NoError(b, err)
	update := []byte(`{
		"Open_vSwitch": {
			"ovs": {"new": ` + newOvsRow("foo") + `}
		},
		"Bridge": {
			"foo": {"new": ` + newBridgeRow("foo") + `}
		}
	}`)
	updateBenchmark(ovs, update, b)
}

func BenchmarkUpdate2(b *testing.B) {
	ovs, err := newOVSDBClient(defDB)
	require.NoError(b, err)
	var s ovsdb.DatabaseSchema
	err = json.Unmarshal([]byte(schema), &s)
	require.NoError(b, err)
	dbModel, err := model.NewDBModel("Open_vSwitch", map[string]model.Model{
		"Bridge":       &Bridge{},
		"Open_vSwitch": &OpenvSwitch{},
	})
	require.NoError(b, err)
	ovs.cache, err = cache.NewTableCache(&s, dbModel, nil)
	require.NoError(b, err)
	update := []byte(`{
		"Open_vSwitch": {
			"ovs": {"new": ` + newOvsRow("foo", "bar") + `}
		},
		"Bridge": {
			"foo": {"new": ` + newBridgeRow("foo") + `},
			"bar": {"new": ` + newBridgeRow("bar") + `}
		}
	}`)
	updateBenchmark(ovs, update, b)
}

func BenchmarkUpdate3(b *testing.B) {
	ovs, err := newOVSDBClient(defDB)
	require.NoError(b, err)
	var s ovsdb.DatabaseSchema
	err = json.Unmarshal([]byte(schema), &s)
	require.NoError(b, err)
	dbModel, err := model.NewDBModel("Open_vSwitch", map[string]model.Model{
		"Bridge":       &Bridge{},
		"Open_vSwitch": &OpenvSwitch{},
	})
	require.NoError(b, err)
	ovs.cache, err = cache.NewTableCache(&s, dbModel, nil)
	require.NoError(b, err)
	update := []byte(`{
		"Open_vSwitch": {
			"ovs": {"new": ` + newOvsRow("foo", "bar", "baz") + `}
		},
		"Bridge": {
			"foo": {"new": ` + newBridgeRow("foo") + `},
			"bar": {"new": ` + newBridgeRow("bar") + `},
			"baz": {"new": ` + newBridgeRow("baz") + `}
		}
	}`)
	updateBenchmark(ovs, update, b)
}

func BenchmarkUpdate5(b *testing.B) {
	ovs, err := newOVSDBClient(defDB)
	require.NoError(b, err)
	var s ovsdb.DatabaseSchema
	err = json.Unmarshal([]byte(schema), &s)
	require.NoError(b, err)
	dbModel, err := model.NewDBModel("Open_vSwitch", map[string]model.Model{
		"Bridge":       &Bridge{},
		"Open_vSwitch": &OpenvSwitch{},
	})
	require.NoError(b, err)
	ovs.cache, err = cache.NewTableCache(&s, dbModel, nil)
	require.NoError(b, err)
	update := []byte(`{
		"Open_vSwitch": {
			"ovs": {"new": ` + newOvsRow("foo", "bar", "baz", "quux", "foofoo") + `}
		},
		"Bridge": {
			"foo": {"new": ` + newBridgeRow("foo") + `},
			"bar": {"new": ` + newBridgeRow("bar") + `},
			"baz": {"new": ` + newBridgeRow("baz") + `},
			"quux": {"new": ` + newBridgeRow("quux") + `},
			"foofoo": {"new": ` + newBridgeRow("foofoo") + `}
		}
	}`)
	updateBenchmark(ovs, update, b)
}

func BenchmarkUpdate8(b *testing.B) {
	ovs, err := newOVSDBClient(defDB)
	require.NoError(b, err)
	var s ovsdb.DatabaseSchema
	err = json.Unmarshal([]byte(schema), &s)
	require.NoError(b, err)
	dbModel, err := model.NewDBModel("Open_vSwitch", map[string]model.Model{
		"Bridge":       &Bridge{},
		"Open_vSwitch": &OpenvSwitch{},
	})
	require.NoError(b, err)
	ovs.cache, err = cache.NewTableCache(&s, dbModel, nil)
	require.NoError(b, err)
	update := []byte(`{
		"Open_vSwitch": {
			"ovs": {"new": ` + newOvsRow("foo", "bar", "baz", "quux", "foofoo", "foobar", "foobaz", "fooquux") + `}
		},
		"Bridge": {
			"foo": {"new": ` + newBridgeRow("foo") + `},
			"bar": {"new": ` + newBridgeRow("bar") + `},
			"baz": {"new": ` + newBridgeRow("baz") + `},
			"quux": {"new": ` + newBridgeRow("quux") + `},
			"foofoo": {"new": ` + newBridgeRow("foofoo") + `},
			"foobar": {"new": ` + newBridgeRow("foobar") + `},
			"foobaz": {"new": ` + newBridgeRow("foobaz") + `},
			"fooquux": {"new": ` + newBridgeRow("fooquux") + `}
		}
	}`)
	updateBenchmark(ovs, update, b)
}

func TestEcho(t *testing.T) {
	req := []interface{}{"hi"}
	var reply []interface{}
	ovs, err := newOVSDBClient(defDB)
	require.NoError(t, err)
	err = ovs.echo(req, &reply)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(req, reply) {
		t.Error("Expected: ", req, " Got: ", reply)
	}
}

func TestUpdate(t *testing.T) {
	ovs, err := newOVSDBClient(defDB)
	require.NoError(t, err)
	var s ovsdb.DatabaseSchema
	err = json.Unmarshal([]byte(schema), &s)
	require.NoError(t, err)
	dbModel, err := model.NewDBModel("Open_vSwitch", map[string]model.Model{
		"Bridge":       &Bridge{},
		"Open_vSwitch": &OpenvSwitch{},
	})
	require.NoError(t, err)
	ovs.cache, err = cache.NewTableCache(&s, dbModel, nil)
	require.NoError(t, err)
	var reply []interface{}
	update := []byte(`{
		"Open_vSwitch": {
			"ovs": {"new": ` + newOvsRow("foo") + `}
		},
		"Bridge": {
			"foo": {"new": ` + newBridgeRow("foo") + `}
		}
	}`)
	params := []json.RawMessage{[]byte(`"v1"`), update}
	err = ovs.update(params, &reply)
	if err != nil {
		t.Error(err)
	}
}

func TestOperationWhenNotConnected(t *testing.T) {
	ovs, err := newOVSDBClient(defDB)
	require.NoError(t, err)
	var s ovsdb.DatabaseSchema
	err = json.Unmarshal([]byte(schema), &s)
	require.NoError(t, err)
	ovs.schema = &s

	tests := []struct {
		name string
		fn   func() error
	}{
		{
			"echo",
			func() error {
				return ovs.Echo(context.Background())
			},
		},
		{
			"transact",
			func() error {
				comment := "this is only a test"
				_, err := ovs.Transact(context.Background(), ovsdb.Operation{Op: ovsdb.OperationComment, Comment: &comment})
				return err
			},
		},
		{
			"monitor/monitor all",
			func() error {
				_, err := ovs.MonitorAll(context.Background())
				return err
			},
		},
		{
			"monitor cancel",
			func() error {
				return ovs.MonitorCancel(context.Background(), "")
			},
		},
	}
	for _, tt := range tests {
		err := tt.fn()
		assert.EqualError(t, err, ErrNotConnected.Error())
	}
}

func TestSetOption(t *testing.T) {
	o, err := newOVSDBClient(defDB)
	require.NoError(t, err)

	o.options, err = newOptions()
	require.NoError(t, err)

	err = o.SetOption(WithEndpoint("tcp::6640"))
	require.NoError(t, err)

	o.rpcClient = &rpc2.Client{}

	err = o.SetOption(WithEndpoint("tcp::6641"))
	assert.EqualError(t, err, "cannot set option when client is connected")
}
