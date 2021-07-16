package server

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/ovn-org/libovsdb/cache"
	"github.com/ovn-org/libovsdb/client"
	"github.com/ovn-org/libovsdb/model"
	"github.com/ovn-org/libovsdb/ovsdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bridgeType is the simplified ORM model of the Bridge table
type bridgeType struct {
	UUID        string            `ovsdb:"_uuid"`
	Name        string            `ovsdb:"name"`
	OtherConfig map[string]string `ovsdb:"other_config"`
	ExternalIds map[string]string `ovsdb:"external_ids"`
	Ports       []string          `ovsdb:"ports"`
	Status      map[string]string `ovsdb:"status"`
}

// ovsType is the simplified ORM model of the Bridge table
type ovsType struct {
	UUID    string   `ovsdb:"_uuid"`
	Bridges []string `ovsdb:"bridges"`
}

func getSchema() (*ovsdb.DatabaseSchema, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(wd, "testdata", "ovslite.json")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	schema, err := ovsdb.SchemaFromFile(f)
	if err != nil {
		return nil, err
	}
	return schema, nil
}

func TestClientServerEcho(t *testing.T) {
	defDB, err := model.NewDBModel("Open_vSwitch", map[string]model.Model{
		"Open_vSwitch": &ovsType{},
		"Bridge":       &bridgeType{}})
	require.Nil(t, err)

	schema, err := getSchema()
	require.Nil(t, err)

	ovsDB := NewInMemoryDatabase(map[string]*model.DBModel{"Open_vSwitch": defDB})

	rand.Seed(time.Now().UnixNano())
	tmpfile := fmt.Sprintf("/tmp/ovsdb-%d.sock", rand.Intn(10000))
	defer os.Remove(tmpfile)
	server, err := NewOvsdbServer(ovsDB, DatabaseModel{
		Model:  defDB,
		Schema: schema,
	})
	require.Nil(t, err)

	go func(t *testing.T, o *OvsdbServer) {
		if err := o.Serve("unix", tmpfile); err != nil {
			t.Error(err)
		}
	}(t, server)
	defer server.Close()
	require.Eventually(t, func() bool {
		return server.Ready()
	}, 1*time.Second, 10*time.Millisecond)

	ovs, err := client.NewOVSDBClient(defDB, client.WithEndpoint(fmt.Sprintf("unix:%s", tmpfile)))
	require.NoError(t, err)
	err = ovs.Connect(context.Background())
	require.NoError(t, err)
	err = ovs.Echo()
	assert.Nil(t, err)
}

func TestClientServerInsert(t *testing.T) {
	defDB, err := model.NewDBModel("Open_vSwitch", map[string]model.Model{
		"Open_vSwitch": &ovsType{},
		"Bridge":       &bridgeType{}})
	require.Nil(t, err)

	schema, err := getSchema()
	require.Nil(t, err)

	ovsDB := NewInMemoryDatabase(map[string]*model.DBModel{"Open_vSwitch": defDB})
	rand.Seed(time.Now().UnixNano())
	tmpfile := fmt.Sprintf("/tmp/ovsdb-%d.sock", rand.Intn(10000))
	defer os.Remove(tmpfile)
	server, err := NewOvsdbServer(ovsDB, DatabaseModel{
		Model:  defDB,
		Schema: schema,
	})
	assert.Nil(t, err)

	go func(t *testing.T, o *OvsdbServer) {
		if err := o.Serve("unix", tmpfile); err != nil {
			t.Error(err)
		}
	}(t, server)
	defer server.Close()
	require.Eventually(t, func() bool {
		return server.Ready()
	}, 1*time.Second, 10*time.Millisecond)

	ovs, err := client.NewOVSDBClient(defDB, client.WithEndpoint(fmt.Sprintf("unix:%s", tmpfile)))
	require.NoError(t, err)
	err = ovs.Connect(context.Background())
	require.NoError(t, err)
	_, err = ovs.MonitorAll()
	require.NoError(t, err)

	bridgeRow := &bridgeType{
		Name:        "foo",
		ExternalIds: map[string]string{"go": "awesome", "docker": "made-for-each-other"},
	}

	ops, err := ovs.Create(bridgeRow)
	require.Nil(t, err)
	reply, err := ovs.Transact(ops...)
	assert.Nil(t, err)
	_, err = ovsdb.CheckOperationResults(reply, ops)
	assert.Nil(t, err)
	uuid := reply[0].UUID.GoUUID
	require.Eventually(t, func() bool {
		br := &bridgeType{UUID: uuid}
		err := ovs.Get(br)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)
}

func TestClientServerMonitor(t *testing.T) {
	defDB, err := model.NewDBModel("Open_vSwitch", map[string]model.Model{
		"Open_vSwitch": &ovsType{},
		"Bridge":       &bridgeType{}})
	if err != nil {
		t.Fatal(err)
	}

	schema, err := getSchema()
	if err != nil {
		t.Fatal(err)
	}

	ovsDB := NewInMemoryDatabase(map[string]*model.DBModel{"Open_vSwitch": defDB})
	rand.Seed(time.Now().UnixNano())
	tmpfile := fmt.Sprintf("/tmp/ovsdb-%d.sock", rand.Intn(10000))
	defer os.Remove(tmpfile)
	server, err := NewOvsdbServer(ovsDB, DatabaseModel{
		Model:  defDB,
		Schema: schema,
	})
	assert.Nil(t, err)

	go func(t *testing.T, o *OvsdbServer) {
		if err := o.Serve("unix", tmpfile); err != nil {
			t.Error(err)
		}
	}(t, server)
	defer server.Close()
	require.Eventually(t, func() bool {
		return server.Ready()
	}, 1*time.Second, 10*time.Millisecond)

	ovs, err := client.NewOVSDBClient(defDB, client.WithEndpoint(fmt.Sprintf("unix:%s", tmpfile)))
	require.NoError(t, err)
	err = ovs.Connect(context.Background())
	require.NoError(t, err)

	ovsRow := &ovsType{
		UUID: "ovs",
	}
	bridgeRow := &bridgeType{
		UUID:        "foo",
		Name:        "foo",
		ExternalIds: map[string]string{"go": "awesome", "docker": "made-for-each-other"},
	}

	seenMutex := sync.RWMutex{}
	seenInsert := false
	seenMutation := false
	seenInitialOvs := false
	ovs.Cache().AddEventHandler(&cache.EventHandlerFuncs{
		AddFunc: func(table string, model model.Model) {
			if table == "Bridge" {
				br := model.(*bridgeType)
				assert.Equal(t, bridgeRow.Name, br.Name)
				assert.Equal(t, bridgeRow.ExternalIds, br.ExternalIds)
				seenMutex.Lock()
				seenInsert = true
				seenMutex.Unlock()
			}
			if table == "Open_vSwitch" {
				seenMutex.Lock()
				seenInitialOvs = true
				seenMutex.Unlock()
			}
		},
		UpdateFunc: func(table string, old, new model.Model) {
			fmt.Println("got an update")
			if table == "Open_vSwitch" {
				ov := new.(*ovsType)
				assert.Equal(t, 1, len(ov.Bridges))
				seenMutex.Lock()
				seenMutation = true
				seenMutex.Unlock()
			}
		},
	})

	var ops []ovsdb.Operation
	ovsOps, err := ovs.Create(ovsRow)
	require.Nil(t, err)
	reply, err := ovs.Transact(ovsOps...)
	require.Nil(t, err)
	_, err = ovsdb.CheckOperationResults(reply, ovsOps)
	require.Nil(t, err)
	require.NotEmpty(t, reply[0].UUID.GoUUID)
	ovsRow.UUID = reply[0].UUID.GoUUID

	_, err = ovs.MonitorAll()
	require.Nil(t, err)
	require.Eventually(t, func() bool {
		seenMutex.RLock()
		defer seenMutex.RUnlock()
		return seenInitialOvs
	}, 1*time.Second, 10*time.Millisecond)

	bridgeOps, err := ovs.Create(bridgeRow)
	require.Nil(t, err)
	ops = append(ops, bridgeOps...)

	mutateOps, err := ovs.Where(ovsRow).Mutate(ovsRow, model.Mutation{
		Field:   &ovsRow.Bridges,
		Mutator: ovsdb.MutateOperationInsert,
		Value:   []string{"foo"},
	})
	require.Nil(t, err)
	ops = append(ops, mutateOps...)

	reply, err = ovs.Transact(ops...)
	require.Nil(t, err)

	_, err = ovsdb.CheckOperationResults(reply, ops)
	assert.Nil(t, err)
	assert.Equal(t, 1, reply[1].Count)

	assert.Eventually(t, func() bool {
		seenMutex.RLock()
		defer seenMutex.RUnlock()
		return seenInsert
	}, 1*time.Second, 10*time.Millisecond)
	assert.Eventually(t, func() bool {
		seenMutex.RLock()
		defer seenMutex.RUnlock()
		return seenMutation
	}, 1*time.Second, 10*time.Millisecond)
}

func TestClientServerInsertAndDelete(t *testing.T) {
	defDB, err := model.NewDBModel("Open_vSwitch", map[string]model.Model{
		"Open_vSwitch": &ovsType{},
		"Bridge":       &bridgeType{}})
	require.Nil(t, err)

	schema, err := getSchema()
	require.Nil(t, err)

	ovsDB := NewInMemoryDatabase(map[string]*model.DBModel{"Open_vSwitch": defDB})
	rand.Seed(time.Now().UnixNano())
	tmpfile := fmt.Sprintf("/tmp/ovsdb-%d.sock", rand.Intn(10000))
	defer os.Remove(tmpfile)
	server, err := NewOvsdbServer(ovsDB, DatabaseModel{
		Model:  defDB,
		Schema: schema,
	})
	assert.Nil(t, err)

	go func(t *testing.T, o *OvsdbServer) {
		if err := o.Serve("unix", tmpfile); err != nil {
			t.Error(err)
		}
	}(t, server)
	defer server.Close()
	require.Eventually(t, func() bool {
		return server.Ready()
	}, 1*time.Second, 10*time.Millisecond)

	ovs, err := client.NewOVSDBClient(defDB, client.WithEndpoint(fmt.Sprintf("unix:%s", tmpfile)))
	require.NoError(t, err)
	err = ovs.Connect(context.Background())
	require.NoError(t, err)
	_, err = ovs.MonitorAll()
	require.NoError(t, err)

	bridgeRow := &bridgeType{
		Name:        "foo",
		ExternalIds: map[string]string{"go": "awesome", "docker": "made-for-each-other"},
	}

	ops, err := ovs.Create(bridgeRow)
	require.Nil(t, err)
	reply, err := ovs.Transact(ops...)
	require.Nil(t, err)
	_, err = ovsdb.CheckOperationResults(reply, ops)
	require.Nil(t, err)

	uuid := reply[0].UUID.GoUUID
	assert.Eventually(t, func() bool {
		br := &bridgeType{UUID: uuid}
		err := ovs.Get(br)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)

	bridgeRow.UUID = uuid
	deleteOp, err := ovs.Where(bridgeRow).Delete()
	require.Nil(t, err)

	reply, err = ovs.Transact(deleteOp...)
	assert.Nil(t, err)
	_, err = ovsdb.CheckOperationResults(reply, ops)
	assert.Nil(t, err)
	assert.Equal(t, 1, reply[0].Count)
}

func TestClientServerInsertDuplicate(t *testing.T) {
	defDB, err := model.NewDBModel("Open_vSwitch", map[string]model.Model{
		"Open_vSwitch": &ovsType{},
		"Bridge":       &bridgeType{}})
	require.Nil(t, err)

	schema, err := getSchema()
	require.Nil(t, err)

	ovsDB := NewInMemoryDatabase(map[string]*model.DBModel{"Open_vSwitch": defDB})
	rand.Seed(time.Now().UnixNano())
	tmpfile := fmt.Sprintf("/tmp/ovsdb-%d.sock", rand.Intn(10000))
	defer os.Remove(tmpfile)
	server, err := NewOvsdbServer(ovsDB, DatabaseModel{
		Model:  defDB,
		Schema: schema,
	})
	assert.Nil(t, err)

	go func(t *testing.T, o *OvsdbServer) {
		if err := o.Serve("unix", tmpfile); err != nil {
			t.Error(err)
		}
	}(t, server)
	defer server.Close()
	require.Eventually(t, func() bool {
		return server.Ready()
	}, 1*time.Second, 10*time.Millisecond)

	ovs, err := client.NewOVSDBClient(defDB, client.WithEndpoint(fmt.Sprintf("unix:%s", tmpfile)))
	require.NoError(t, err)
	err = ovs.Connect(context.Background())
	require.NoError(t, err)

	bridgeRow := &bridgeType{
		Name:        "foo",
		ExternalIds: map[string]string{"go": "awesome", "docker": "made-for-each-other"},
	}

	ops, err := ovs.Create(bridgeRow)
	require.Nil(t, err)
	reply, err := ovs.Transact(ops...)
	require.Nil(t, err)
	_, err = ovsdb.CheckOperationResults(reply, ops)
	require.Nil(t, err)

	// duplicate
	reply, err = ovs.Transact(ops...)
	require.Nil(t, err)
	opErrs, err := ovsdb.CheckOperationResults(reply, ops)
	require.Error(t, err)
	require.Error(t, opErrs[0])
	require.IsTypef(t, &ovsdb.ConstraintViolation{}, opErrs[0], opErrs[0].Error())
}

func TestClientServerInsertAndUpdate(t *testing.T) {
	defDB, err := model.NewDBModel("Open_vSwitch", map[string]model.Model{
		"Open_vSwitch": &ovsType{},
		"Bridge":       &bridgeType{}})
	require.Nil(t, err)

	schema, err := getSchema()
	require.Nil(t, err)

	ovsDB := NewInMemoryDatabase(map[string]*model.DBModel{"Open_vSwitch": defDB})
	rand.Seed(time.Now().UnixNano())
	tmpfile := fmt.Sprintf("/tmp/ovsdb-%d.sock", rand.Intn(10000))
	defer os.Remove(tmpfile)
	server, err := NewOvsdbServer(ovsDB, DatabaseModel{
		Model:  defDB,
		Schema: schema,
	})
	assert.Nil(t, err)

	go func(t *testing.T, o *OvsdbServer) {
		if err := o.Serve("unix", tmpfile); err != nil {
			t.Error(err)
		}
	}(t, server)
	defer server.Close()
	require.Eventually(t, func() bool {
		return server.Ready()
	}, 1*time.Second, 10*time.Millisecond)

	ovs, err := client.NewOVSDBClient(defDB, client.WithEndpoint(fmt.Sprintf("unix:%s", tmpfile)))
	require.NoError(t, err)
	err = ovs.Connect(context.Background())
	require.NoError(t, err)
	defer ovs.Disconnect()

	_, err = ovs.MonitorAll()
	require.NoError(t, err)

	bridgeRow := &bridgeType{
		Name:        "br-update",
		ExternalIds: map[string]string{"go": "awesome", "docker": "made-for-each-other"},
	}

	ops, err := ovs.Create(bridgeRow)
	require.NoError(t, err)
	reply, err := ovs.Transact(ops...)
	require.NoError(t, err)
	_, err = ovsdb.CheckOperationResults(reply, ops)
	require.NoError(t, err)

	uuid := reply[0].UUID.GoUUID
	assert.Eventually(t, func() bool {
		br := &bridgeType{UUID: uuid}
		err := ovs.Get(br)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)

	// try to modify immutable field
	bridgeRow.UUID = uuid
	bridgeRow.Name = "br-update2"
	ops, err = ovs.Where(bridgeRow).Update(bridgeRow)
	require.NoError(t, err)
	reply, err = ovs.Transact(ops...)
	require.NoError(t, err)
	_, err = ovsdb.CheckOperationResults(reply, ops)
	require.Error(t, err)
	bridgeRow.Name = "br-update"

	/* FIXME: https://github.com/ovn-org/libovsdb/issues/203
	// update many fields
	bridgeRow.UUID = uuid
	bridgeRow.Name = "br-update"
	bridgeRow.ExternalIds["baz"] = "foobar"
	bridgeRow.OtherConfig = map[string]string{"foo": "bar"}
	ops, err = ovs.Where(bridgeRow).Update(bridgeRow)
	require.NoError(t, err)
	reply, err = ovs.Transact(ops...)
	require.NoError(t, err)
	opErrs, err := ovsdb.CheckOperationResults(reply, ops)
	require.NoErrorf(t, err, "%+v", opErrs)

	require.Eventually(t, func() bool {
		br := &bridgeType{UUID: uuid}
		err = ovs.Get(br)
		if err != nil {
			return false
		}
		return reflect.DeepEqual(br, bridgeRow)
	}, 2*time.Second, 50*time.Millisecond)
	*/

	newExternalIds := map[string]string{"foo": "bar"}
	bridgeRow.ExternalIds = newExternalIds
	ops, err = ovs.Where(bridgeRow).Update(bridgeRow, &bridgeRow.ExternalIds)
	require.NoError(t, err)
	reply, err = ovs.Transact(ops...)
	require.NoError(t, err)
	_, err = ovsdb.CheckOperationResults(reply, ops)
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		br := &bridgeType{UUID: uuid}
		err = ovs.Get(br)
		if err != nil {
			return false
		}
		return reflect.DeepEqual(br, bridgeRow)
	}, 2*time.Second, 500*time.Millisecond)
}
