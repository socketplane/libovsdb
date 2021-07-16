package ovs

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/ovn-org/libovsdb/cache"
	"github.com/ovn-org/libovsdb/client"
	"github.com/ovn-org/libovsdb/model"
	"github.com/ovn-org/libovsdb/ovsdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// OVSIntegrationSuite runs tests against a real Open vSwitch instance
type OVSIntegrationSuite struct {
	suite.Suite
	pool     *dockertest.Pool
	resource *dockertest.Resource
	client   client.Client
}

func (suite *OVSIntegrationSuite) SetupSuite() {
	var err error
	suite.pool, err = dockertest.NewPool("")
	require.NoError(suite.T(), err)

	tag := os.Getenv("OVS_IMAGE_TAG")
	if tag == "" {
		tag = "latest"
	}

	options := &dockertest.RunOptions{
		Repository:   "libovsdb/ovs",
		Tag:          tag,
		ExposedPorts: []string{"6640/tcp"},
		Tty:          true,
	}
	hostConfig := func(config *docker.HostConfig) {
		// set AutoRemove to true so that stopped container goes away by itself
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
	}

	suite.resource, err = suite.pool.RunWithOptions(options, hostConfig)
	require.NoError(suite.T(), err)

	// set expiry to 30 seconds so containers are cleaned up on test panic
	err = suite.resource.Expire(30)
	require.NoError(suite.T(), err)

	// let the container start before we attempt connection
	time.Sleep(5 * time.Second)

	err = suite.pool.Retry(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		endpoint := fmt.Sprintf("tcp::%s", suite.resource.GetPort("6640/tcp"))
		ovs, err := client.NewOVSDBClient(
			defDB,
			client.WithEndpoint(endpoint),
		)
		if err != nil {
			return err
		}
		err = ovs.Connect(ctx)
		if err != nil {
			return err
		}
		suite.client = ovs
		return nil
	})
	require.NoError(suite.T(), err)

	// give ovsdb-server some time to start up

	_, err = suite.client.MonitorAll()
	require.NoError(suite.T(), err)
}

func (suite *OVSIntegrationSuite) TearDownSuite() {
	if suite.client != nil {
		suite.client.Close()
		suite.client = nil
	}
	err := suite.pool.Purge(suite.resource)
	require.NoError(suite.T(), err)
}

func TestOVSIntegrationTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	suite.Run(t, new(OVSIntegrationSuite))
}

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

var defDB, _ = model.NewDBModel("Open_vSwitch", map[string]model.Model{
	"Open_vSwitch": &ovsType{},
	"Bridge":       &bridgeType{}})

func (suite *OVSIntegrationSuite) TestConnectReconnect() {
	assert.True(suite.T(), suite.client.Connected())
	err := suite.client.Echo()
	require.NoError(suite.T(), err)

	bridgeName := "br-discoreco"
	brChan := make(chan *bridgeType)
	suite.client.Cache().AddEventHandler(&cache.EventHandlerFuncs{
		AddFunc: func(table string, model model.Model) {
			br, ok := model.(*bridgeType)
			if !ok {
				return
			}
			if br.Name == bridgeName {
				brChan <- br
			}
		},
	})

	bridgeUUID, err := suite.createBridge(bridgeName)
	require.NoError(suite.T(), err)
	br := <-brChan

	// make another connect call, this should return without error as we're already connected
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = suite.client.Connect(ctx)
	require.NoError(suite.T(), err)

	disconnectNotification := suite.client.DisconnectNotify()
	notified := make(chan struct{})
	ready := make(chan struct{})

	go func() {
		ready <- struct{}{}
		<-disconnectNotification
		notified <- struct{}{}
	}()

	<-ready
	suite.client.Disconnect()

	select {
	case <-notified:
		// got notification
	case <-time.After(5 * time.Second):
		suite.T().Fatal("expected a disconnect notification but didn't receive one")
	}

	assert.Equal(suite.T(), false, suite.client.Connected())

	err = suite.client.Echo()
	require.EqualError(suite.T(), err, client.ErrNotConnected.Error())

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = suite.client.Connect(ctx)
	require.NoError(suite.T(), err)

	br = &bridgeType{
		UUID: bridgeUUID,
	}

	// assert cache has been purged
	err = suite.client.Get(br)
	require.Error(suite.T(), err, client.ErrNotFound)

	err = suite.client.Echo()
	assert.NoError(suite.T(), err)

	_, err = suite.client.MonitorAll()
	require.NoError(suite.T(), err)

	// assert cache has been re-populated
	require.Eventually(suite.T(), func() bool {
		err := suite.client.Get(br)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)

}

func (suite *OVSIntegrationSuite) TestWithReconnect() {
	assert.Equal(suite.T(), true, suite.client.Connected())
	err := suite.client.Echo()
	require.NoError(suite.T(), err)

	// Disconnect client
	suite.client.Disconnect()

	require.Eventually(suite.T(), func() bool {
		return !suite.client.Connected()
	}, 5*time.Second, 1*time.Second)

	// Reconfigure
	err = suite.client.SetOption(
		client.WithReconnect(2*time.Second, backoff.NewExponentialBackOff()),
	)
	require.NoError(suite.T(), err)

	// Connect (again)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = suite.client.Connect(ctx)
	require.NoError(suite.T(), err)

	// make another connect call, this should return without error as we're already connected
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = suite.client.Connect(ctx)
	require.NoError(suite.T(), err)

	// check the connection is working
	err = suite.client.Echo()
	require.NoError(suite.T(), err)

	// check the cache is purged
	require.True(suite.T(), suite.client.Cache().Table("Bridge").Len() == 0)

	// set up the monitor again
	_, err = suite.client.MonitorAll()
	require.NoError(suite.T(), err)

	// add a bridge and verify our handler gets called
	bridgeName := "recon-b4"
	brChan := make(chan *bridgeType)
	suite.client.Cache().AddEventHandler(&cache.EventHandlerFuncs{
		AddFunc: func(table string, model model.Model) {
			br, ok := model.(*bridgeType)
			if !ok {
				return
			}
			if strings.HasPrefix(br.Name, "recon-") {
				brChan <- br
			}
		},
	})

	_, err = suite.createBridge(bridgeName)
	require.NoError(suite.T(), err)
	br := <-brChan
	require.Equal(suite.T(), bridgeName, br.Name)

	// trigger reconnect
	suite.client.Disconnect()

	// check that we are automatically reconnected
	require.Eventually(suite.T(), func() bool {
		return suite.client.Connected()
	}, 2*time.Second, 500*time.Millisecond)

	err = suite.client.Echo()
	require.NoError(suite.T(), err)

	// check our original bridge is in the cache
	err = suite.client.Get(br)
	require.NoError(suite.T(), err)

	// create a new bridge to ensure the monitor and cache handler is still working
	bridgeName = "recon-after"
	_, err = suite.createBridge(bridgeName)
	require.NoError(suite.T(), err)

LOOP:
	for {
		select {
		case <-time.After(2 * time.Second):
			suite.T().Fatal("timed out waiting for bridge")
		case b := <-brChan:
			if b.Name == bridgeName {
				break LOOP
			}
		}
	}

	// set up a disconnect notification
	disconnectNotification := suite.client.DisconnectNotify()
	notified := make(chan struct{})
	ready := make(chan struct{})

	go func() {
		ready <- struct{}{}
		<-disconnectNotification
		notified <- struct{}{}
	}()

	<-ready
	// close the connection
	suite.client.Close()

	select {
	case <-notified:
		// got notification
	case <-time.After(5 * time.Second):
		suite.T().Fatal("expected a disconnect notification but didn't receive one")
	}

	assert.Equal(suite.T(), false, suite.client.Connected())

	err = suite.client.Echo()
	require.EqualError(suite.T(), err, client.ErrNotConnected.Error())

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = suite.client.Connect(ctx)
	require.NoError(suite.T(), err)

	err = suite.client.Echo()
	assert.NoError(suite.T(), err)

	_, err = suite.client.MonitorAll()
	require.NoError(suite.T(), err)
}

func (suite *OVSIntegrationSuite) TestInsertTransactIntegration() {
	bridgeName := "gopher-br7"
	uuid, err := suite.createBridge(bridgeName)
	require.NoError(suite.T(), err)
	require.Eventually(suite.T(), func() bool {
		br := &bridgeType{UUID: uuid}
		err := suite.client.Get(br)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)
}

func (suite *OVSIntegrationSuite) TestInsertAndDeleteTransactIntegration() {
	bridgeName := "gopher-br5"
	bridgeUUID, err := suite.createBridge(bridgeName)
	require.NoError(suite.T(), err)

	require.Eventually(suite.T(), func() bool {
		br := &bridgeType{UUID: bridgeUUID}
		err := suite.client.Get(br)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)

	deleteOp, err := suite.client.Where(&bridgeType{Name: bridgeName}).Delete()
	require.NoError(suite.T(), err)

	ovsRow := ovsType{}
	delMutateOp, err := suite.client.WhereCache(func(*ovsType) bool { return true }).
		Mutate(&ovsRow, model.Mutation{
			Field:   &ovsRow.Bridges,
			Mutator: ovsdb.MutateOperationDelete,
			Value:   []string{bridgeUUID},
		})

	require.NoError(suite.T(), err)

	delOperations := append(deleteOp, delMutateOp...)
	delReply, err := suite.client.Transact(delOperations...)
	require.NoError(suite.T(), err)

	delOperationErrs, err := ovsdb.CheckOperationResults(delReply, delOperations)
	if err != nil {
		for _, oe := range delOperationErrs {
			suite.T().Error(oe)
		}
		suite.T().Fatal(err)
	}
}

func (suite *OVSIntegrationSuite) TestTableSchemaValidationIntegration() {
	operation := ovsdb.Operation{
		Op:    "insert",
		Table: "InvalidTable",
		Row:   ovsdb.Row(map[string]interface{}{"name": "docker-ovs"}),
	}
	_, err := suite.client.Transact(operation)
	assert.Error(suite.T(), err)
}

func (suite *OVSIntegrationSuite) TestColumnSchemaInRowValidationIntegration() {
	operation := ovsdb.Operation{
		Op:    "insert",
		Table: "Bridge",
		Row:   ovsdb.Row(map[string]interface{}{"name": "docker-ovs", "invalid_column": "invalid_column"}),
	}

	_, err := suite.client.Transact(operation)
	assert.Error(suite.T(), err)
}

func (suite *OVSIntegrationSuite) TestColumnSchemaInMultipleRowsValidationIntegration() {
	invalidBridge := ovsdb.Row(map[string]interface{}{"invalid_column": "invalid_column"})
	bridge := ovsdb.Row(map[string]interface{}{"name": "docker-ovs"})
	rows := []ovsdb.Row{invalidBridge, bridge}

	operation := ovsdb.Operation{
		Op:    "insert",
		Table: "Bridge",
		Rows:  rows,
	}
	_, err := suite.client.Transact(operation)
	assert.Error(suite.T(), err)
}

func (suite *OVSIntegrationSuite) TestColumnSchemaValidationIntegration() {
	operation := ovsdb.Operation{
		Op:      "select",
		Table:   "Bridge",
		Columns: []string{"name", "invalidColumn"},
	}
	_, err := suite.client.Transact(operation)
	assert.Error(suite.T(), err)
}

func (suite *OVSIntegrationSuite) TestMonitorCancelIntegration() {
	requests := make(map[string]ovsdb.MonitorRequest)
	requests["Bridge"] = ovsdb.MonitorRequest{
		Columns: []string{"name"},
		Select:  ovsdb.NewDefaultMonitorSelect(),
	}

	monitorID, err := suite.client.Monitor(
		suite.client.NewTableMonitor(&ovsType{}),
		suite.client.NewTableMonitor(&bridgeType{}),
	)
	require.NoError(suite.T(), err)

	err = suite.client.MonitorCancel(monitorID)
	assert.NoError(suite.T(), err)

	uuid, err := suite.createBridge("br-monitor")
	require.NoError(suite.T(), err)

	assert.Eventually(suite.T(), func() bool {
		br := &bridgeType{UUID: uuid}
		err = suite.client.Get(br)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)
}

func (suite *OVSIntegrationSuite) TestInsertDuplicateTransactIntegration() {
	uuid, err := suite.createBridge("br-dup")
	require.NoError(suite.T(), err)

	require.Eventually(suite.T(), func() bool {
		br := &bridgeType{UUID: uuid}
		err := suite.client.Get(br)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)

	_, err = suite.createBridge("br-dup")
	assert.Error(suite.T(), err)
	assert.IsType(suite.T(), &ovsdb.ConstraintViolation{}, err)
}

func (suite *OVSIntegrationSuite) TestUpdate() {
	uuid, err := suite.createBridge("br-update")
	require.NoError(suite.T(), err)

	require.Eventually(suite.T(), func() bool {
		br := &bridgeType{UUID: uuid}
		err := suite.client.Get(br)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)

	bridgeRow := &bridgeType{UUID: uuid}
	err = suite.client.Get(bridgeRow)
	require.NoError(suite.T(), err)

	// FIXME: https://github.com/ovn-org/libovsdb/issues/203
	/*
		// update many fields
		bridgeRow.UUID = uuid
		bridgeRow.ExternalIds["baz"] = "foobar"
		bridgeRow.OtherConfig = map[string]string{"foo": "bar"}
		ops, err = suite.client.Where(bridgeRow).Update(bridgeRow)
		require.NoError(suite.T(), err)
		reply, err = suite.client.Transact(ops...)
		require.NoError(suite.T(), err)
		opErrs, err := ovsdb.CheckOperationResults(reply, ops)
		require.NoErrorf(suite.T(), err, "%+v", opErrs)

		require.Eventually(suite.T(), func() bool {
			br := &bridgeType{UUID: uuid}
			err = suite.client.Get(br)
			if err != nil {
				return false
			}
			return reflect.DeepEqual(bridgeRow, br)
		}, 2*time.Second, 500*time.Millisecond)

	*/

	// try to modify immutable field
	bridgeRow.Name = "br-update2"
	ops, err := suite.client.Where(bridgeRow).Update(bridgeRow)
	require.NoError(suite.T(), err)
	reply, err := suite.client.Transact(ops...)
	require.NoError(suite.T(), err)
	_, err = ovsdb.CheckOperationResults(reply, ops)
	require.Error(suite.T(), err)
	// set name back again
	bridgeRow.Name = "br-update"

	newExternalIds := map[string]string{"foo": "bar"}
	bridgeRow.ExternalIds = newExternalIds
	ops, err = suite.client.Where(bridgeRow).Update(bridgeRow, &bridgeRow.ExternalIds)
	require.NoError(suite.T(), err)
	reply, err = suite.client.Transact(ops...)
	require.NoError(suite.T(), err)
	_, err = ovsdb.CheckOperationResults(reply, ops)
	require.NoError(suite.T(), err)

	assert.Eventually(suite.T(), func() bool {
		br := &bridgeType{UUID: uuid}
		err = suite.client.Get(br)
		if err != nil {
			return false
		}
		return reflect.DeepEqual(bridgeRow, br)
	}, 2*time.Second, 500*time.Millisecond)
}

func (suite *OVSIntegrationSuite) createBridge(bridgeName string) (string, error) {
	// NamedUUID is used to add multiple related Operations in a single Transact operation
	namedUUID := "gopher"
	br := bridgeType{
		UUID: namedUUID,
		Name: bridgeName,
		ExternalIds: map[string]string{
			"go":     "awesome",
			"docker": "made-for-each-other",
		},
	}

	insertOp, err := suite.client.Create(&br)
	require.NoError(suite.T(), err)

	// Inserting a Bridge row in Bridge table requires mutating the open_vswitch table.
	ovsRow := ovsType{}
	mutateOp, err := suite.client.WhereCache(func(*ovsType) bool { return true }).
		Mutate(&ovsRow, model.Mutation{
			Field:   &ovsRow.Bridges,
			Mutator: ovsdb.MutateOperationInsert,
			Value:   []string{namedUUID},
		})
	require.NoError(suite.T(), err)

	operations := append(insertOp, mutateOp...)
	reply, err := suite.client.Transact(operations...)
	require.NoError(suite.T(), err)

	_, err = ovsdb.CheckOperationResults(reply, operations)
	return reply[0].UUID.GoUUID, err
}
