package server

import (
	"testing"

	"github.com/google/uuid"
	"github.com/ovn-org/libovsdb/mapper"
	"github.com/ovn-org/libovsdb/model"
	"github.com/ovn-org/libovsdb/ovsdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMutateOp(t *testing.T) {
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
	o, err := NewOvsdbServer(ovsDB, DatabaseModel{
		Model: defDB, Schema: schema})
	require.Nil(t, err)

	ovsUUID := uuid.NewString()
	bridgeUUID := uuid.NewString()

	m := mapper.NewMapper(schema)

	ovs := ovsType{}
	ovsRow, err := m.NewRow("Open_vSwitch", &ovs)
	require.Nil(t, err)

	bridge := bridgeType{
		Name: "foo",
	}
	bridgeRow, err := m.NewRow("Bridge", &bridge)
	require.Nil(t, err)

	res, updates := o.Insert("Open_vSwitch", "Open_vSwitch", ovsUUID, ovsRow)
	_, err = ovsdb.CheckOperationResults([]ovsdb.OperationResult{res}, []ovsdb.Operation{{Op: "insert"}})
	require.Nil(t, err)

	res, update2 := o.Insert("Open_vSwitch", "Bridge", bridgeUUID, bridgeRow)
	_, err = ovsdb.CheckOperationResults([]ovsdb.OperationResult{res}, []ovsdb.Operation{{Op: "insert"}})
	require.Nil(t, err)

	updates.Merge(update2)
	err = o.db.Commit("Open_vSwitch", updates)
	require.NoError(t, err)

	gotResult, gotUpdate := o.Mutate(
		"Open_vSwitch",
		"Open_vSwitch",
		[]ovsdb.Condition{
			ovsdb.NewCondition("_uuid", ovsdb.ConditionEqual, ovsdb.UUID{GoUUID: ovsUUID}),
		},
		[]ovsdb.Mutation{
			*ovsdb.NewMutation("bridges", ovsdb.MutateOperationInsert, ovsdb.UUID{GoUUID: bridgeUUID}),
		},
	)
	assert.Equal(t, ovsdb.OperationResult{Count: 1}, gotResult)

	bridgeSet, err := ovsdb.NewOvsSet([]ovsdb.UUID{{GoUUID: bridgeUUID}})
	assert.Nil(t, err)
	assert.Equal(t, ovsdb.TableUpdates{
		"Open_vSwitch": ovsdb.TableUpdate{
			ovsUUID: &ovsdb.RowUpdate{
				Old: &ovsdb.Row{
					"_uuid": ovsdb.UUID{GoUUID: ovsUUID},
				},
				New: &ovsdb.Row{
					"_uuid":   ovsdb.UUID{GoUUID: ovsUUID},
					"bridges": bridgeSet,
				},
			},
		},
	}, gotUpdate)
}
