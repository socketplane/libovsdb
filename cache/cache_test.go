package cache

import (
	"testing"

	"encoding/json"

	"github.com/ovn-org/libovsdb/mapper"
	"github.com/ovn-org/libovsdb/model"
	"github.com/ovn-org/libovsdb/ovsdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testModel struct {
	UUID string `ovsdb:"_uuid"`
	Foo  string `ovsdb:"foo"`
	Bar  string `ovsdb:"bar"`
}

func TestRowCache_Row(t *testing.T) {

	type fields struct {
		cache map[string]model.Model
	}
	type args struct {
		uuid string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   model.Model
	}{
		{
			"returns a row that exists",
			fields{cache: map[string]model.Model{"test": &testModel{}}},
			args{uuid: "test"},
			&testModel{},
		},
		{
			"returns a nil for a row that does not exist",
			fields{cache: map[string]model.Model{"test": &testModel{}}},
			args{uuid: "foo"},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &RowCache{
				cache: tt.fields.cache,
			}
			got := r.Row(tt.args.uuid)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRowCache_Rows(t *testing.T) {
	type fields struct {
		cache map[string]model.Model
	}
	tests := []struct {
		name   string
		fields fields
		want   []string
	}{
		{
			"returns a rows that exist",
			fields{cache: map[string]model.Model{"test1": &testModel{}, "test2": &testModel{}, "test3": &testModel{}}},
			[]string{"test1", "test2", "test3"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &RowCache{
				cache: tt.fields.cache,
			}
			got := r.Rows()
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}

func TestRowCacheCreate(t *testing.T) {
	var schema ovsdb.DatabaseSchema
	db, err := model.NewDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	require.Nil(t, err)
	err = json.Unmarshal([]byte(`
		 {"name": "TestDB",
		  "tables": {
		    "Open_vSwitch": {
			  "indexes": [["foo"]],
		      "columns": {
		        "foo": {
			  "type": "string"
			},
			"bar": {
				"type": "string"
			  }
		      }
		    }
		 }
	     }
	`), &schema)
	require.Nil(t, err)
	testData := Data{
		"Open_vSwitch": map[string]model.Model{"bar": &testModel{Foo: "bar"}},
	}
	tc, err := NewTableCache(&schema, db, testData)
	require.Nil(t, err)

	tests := []struct {
		name       string
		uuid       string
		model      *testModel
		checkIndex bool
		wantErr    bool
	}{
		{
			"inserts a new row",
			"foo",
			&testModel{Foo: "foo"},
			true,
			false,
		},
		{
			"error duplicate uuid",
			"bar",
			&testModel{Foo: "foo"},
			true,
			true,
		},
		{
			"error duplicate index",
			"baz",
			&testModel{Foo: "bar"},
			true,
			true,
		},
		{
			"error duplicate uuid, no index check",
			"bar",
			&testModel{Foo: "bar"},
			false,
			true,
		},
		{
			"no error duplicate index",
			"baz",
			&testModel{Foo: "bar"},
			false,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := tc.Table("Open_vSwitch")
			require.NotNil(t, rc)
			err := rc.Create(tt.uuid, tt.model, tt.checkIndex)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, tt.uuid, rc.indexes["foo"][tt.model.Foo])
			}
		})
	}
}

func TestRowCacheCreateMultiIndex(t *testing.T) {
	var schema ovsdb.DatabaseSchema
	db, err := model.NewDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	require.Nil(t, err)
	err = json.Unmarshal([]byte(`
		 {"name": "TestDB",
		  "tables": {
		    "Open_vSwitch": {
			  "indexes": [["foo", "bar"]],
		      "columns": {
		        "foo": {
			  "type": "string"
			},
			"bar": {
				"type": "string"
			  }
		      }
		    }
		 }
	     }
	`), &schema)
	require.Nil(t, err)
	tSchema := schema.Table("Open_vSwitch")
	testData := Data{
		"Open_vSwitch": map[string]model.Model{"bar": &testModel{Foo: "bar", Bar: "bar"}},
	}
	tc, err := NewTableCache(&schema, db, testData)
	require.Nil(t, err)
	tests := []struct {
		name               string
		uuid               string
		model              *testModel
		wantErr            bool
		wantIndexExistsErr bool
	}{
		{
			"inserts a new row",
			"foo",
			&testModel{Foo: "foo", Bar: "foo"},
			false,
			false,
		},
		{
			"error duplicate uuid",
			"bar",
			&testModel{Foo: "bar", Bar: "bar"},
			true,
			false,
		},
		{
			"error duplicate index",
			"baz",
			&testModel{Foo: "foo", Bar: "foo"},
			true,
			true,
		},
		{
			"new row with one duplicate value",
			"baz",
			&testModel{Foo: "foo", Bar: "bar"},
			false,
			false,
		},
		{
			"new row with other duplicate value",
			"quux",
			&testModel{Foo: "bar", Bar: "baz"},
			false,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := tc.Table("Open_vSwitch")
			require.NotNil(t, rc)
			err := rc.Create(tt.uuid, tt.model, true)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantIndexExistsErr {
					assert.IsType(t, &ErrIndexExists{}, err)
				}
			} else {
				assert.Nil(t, err)
				mapperInfo, err := mapper.NewInfo(tSchema, tt.model)
				require.Nil(t, err)
				h, err := valueFromIndex(mapperInfo, newIndex("foo", "bar"))
				require.Nil(t, err)
				assert.Equal(t, tt.uuid, rc.indexes["foo,bar"][h])
			}
		})
	}
}

func TestRowCacheUpdate(t *testing.T) {
	var schema ovsdb.DatabaseSchema
	db, err := model.NewDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	require.Nil(t, err)
	err = json.Unmarshal([]byte(`
		 {"name": "TestDB",
		  "tables": {
		    "Open_vSwitch": {
			  "indexes": [["foo"]],
		      "columns": {
		        "foo": {
			  "type": "string"
			},
			"bar": {
				"type": "string"
			  }
		      }
		    }
		 }
	     }
	`), &schema)
	require.Nil(t, err)
	testData := Data{
		"Open_vSwitch": map[string]model.Model{
			"bar":    &testModel{Foo: "bar"},
			"foobar": &testModel{Foo: "foobar"},
		},
	}
	tc, err := NewTableCache(&schema, db, testData)
	require.Nil(t, err)

	tests := []struct {
		name       string
		uuid       string
		model      *testModel
		checkIndex bool
		wantErr    bool
	}{
		{
			"error if row does not exist",
			"foo",
			&testModel{Foo: "foo"},
			true,
			true,
		},
		{
			"update",
			"bar",
			&testModel{Foo: "baz"},
			true,
			false,
		},
		{
			"error new index would cause duplicate",
			"bar",
			&testModel{Foo: "foobar"},
			true,
			true,
		},
		{
			"no error new index would cause duplicate",
			"bar",
			&testModel{Foo: "foobar"},
			false,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := tc.Table("Open_vSwitch")
			require.NotNil(t, rc)
			err := rc.Update(tt.uuid, tt.model, tt.checkIndex)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, tt.uuid, rc.indexes["foo"][tt.model.Foo])
			}
		})
	}
}

func TestRowCacheUpdateMultiIndex(t *testing.T) {
	var schema ovsdb.DatabaseSchema
	db, err := model.NewDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	require.Nil(t, err)
	err = json.Unmarshal([]byte(`
		 {"name": "TestDB",
		  "tables": {
		    "Open_vSwitch": {
			  "indexes": [["foo", "bar"]],
		      "columns": {
		        "foo": {
			  "type": "string"
			},
			"bar": {
				"type": "string"
			  }
		      }
		    }
		 }
	     }
	`), &schema)
	tSchema := schema.Table("Open_vSwitch")
	require.Nil(t, err)
	testData := Data{
		"Open_vSwitch": map[string]model.Model{
			"bar":    &testModel{Foo: "bar", Bar: "bar"},
			"foobar": &testModel{Foo: "foobar", Bar: "foobar"},
		},
	}
	tc, err := NewTableCache(&schema, db, testData)
	require.Nil(t, err)

	tests := []struct {
		name    string
		uuid    string
		model   *testModel
		wantErr bool
	}{
		{
			"error if row does not exist",
			"foo",
			&testModel{Foo: "foo", Bar: "foo"},
			true,
		},
		{
			"update both index cols",
			"bar",
			&testModel{Foo: "baz", Bar: "baz"},
			false,
		},
		{
			"update single index col",
			"bar",
			&testModel{Foo: "baz", Bar: "quux"},
			false,
		},
		{
			"error new index would cause duplicate",
			"baz",
			&testModel{Foo: "foobar", Bar: "foobar"},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := tc.Table("Open_vSwitch")
			require.NotNil(t, rc)
			err := rc.Update(tt.uuid, tt.model, true)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.Nil(t, err)
				mapperInfo, err := mapper.NewInfo(tSchema, tt.model)
				require.Nil(t, err)
				h, err := valueFromIndex(mapperInfo, newIndex("foo", "bar"))
				require.Nil(t, err)
				assert.Equal(t, tt.uuid, rc.indexes["foo,bar"][h])
			}
		})
	}
}

func TestRowCacheDelete(t *testing.T) {
	var schema ovsdb.DatabaseSchema
	db, err := model.NewDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	require.Nil(t, err)
	err = json.Unmarshal([]byte(`
		 {"name": "TestDB",
		  "tables": {
		    "Open_vSwitch": {
			  "indexes": [["foo"]],
		      "columns": {
		        "foo": {
			  "type": "string"
			},
			"bar": {
				"type": "string"
			  }
		      }
		    }
		 }
	     }
	`), &schema)
	require.Nil(t, err)
	testData := Data{
		"Open_vSwitch": map[string]model.Model{
			"bar": &testModel{Foo: "bar"},
		},
	}
	tc, err := NewTableCache(&schema, db, testData)
	require.Nil(t, err)

	tests := []struct {
		name    string
		uuid    string
		model   *testModel
		wantErr bool
	}{
		{
			"deletes a row",
			"bar",
			&testModel{Foo: "bar"},
			false,
		},
		{
			"error if row does not exist",
			"foobar",
			&testModel{Foo: "bar"},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := tc.Table("Open_vSwitch")
			require.NotNil(t, rc)
			err := rc.Delete(tt.uuid)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.Nil(t, err)
				assert.Equal(t, "", rc.indexes["foo"][tt.model.Foo])
			}
		})
	}
}

func TestEventHandlerFuncs_OnAdd(t *testing.T) {
	calls := 0
	type fields struct {
		AddFunc    func(table string, row model.Model)
		UpdateFunc func(table string, old model.Model, new model.Model)
		DeleteFunc func(table string, row model.Model)
	}
	type args struct {
		table string
		row   model.Model
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			"doesn't call nil function",
			fields{nil, nil, nil},
			args{"testTable", &testModel{}},
		},
		{
			"calls onadd function",
			fields{func(string, model.Model) {
				calls++
			}, nil, nil},
			args{"testTable", &testModel{}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &EventHandlerFuncs{
				AddFunc:    tt.fields.AddFunc,
				UpdateFunc: tt.fields.UpdateFunc,
				DeleteFunc: tt.fields.DeleteFunc,
			}
			e.OnAdd(tt.args.table, tt.args.row)
			if e.AddFunc != nil {
				assert.Equal(t, 1, calls)
			}
		})
	}
}

func TestEventHandlerFuncs_OnUpdate(t *testing.T) {
	calls := 0
	type fields struct {
		AddFunc    func(table string, row model.Model)
		UpdateFunc func(table string, old model.Model, new model.Model)
		DeleteFunc func(table string, row model.Model)
	}
	type args struct {
		table string
		old   model.Model
		new   model.Model
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			"doesn't call nil function",
			fields{nil, nil, nil},
			args{"testTable", &testModel{}, &testModel{}},
		},
		{
			"calls onupdate function",
			fields{nil, func(string, model.Model, model.Model) {
				calls++
			}, nil},
			args{"testTable", &testModel{}, &testModel{}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &EventHandlerFuncs{
				AddFunc:    tt.fields.AddFunc,
				UpdateFunc: tt.fields.UpdateFunc,
				DeleteFunc: tt.fields.DeleteFunc,
			}
			e.OnUpdate(tt.args.table, tt.args.old, tt.args.new)
			if e.UpdateFunc != nil {
				assert.Equal(t, 1, calls)
			}
		})
	}
}

func TestEventHandlerFuncs_OnDelete(t *testing.T) {
	calls := 0
	type fields struct {
		AddFunc    func(table string, row model.Model)
		UpdateFunc func(table string, old model.Model, new model.Model)
		DeleteFunc func(table string, row model.Model)
	}
	type args struct {
		table string
		row   model.Model
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			"doesn't call nil function",
			fields{nil, nil, nil},
			args{"testTable", &testModel{}},
		},
		{
			"calls ondelete function",
			fields{nil, nil, func(string, model.Model) {
				calls++
			}},
			args{"testTable", &testModel{}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &EventHandlerFuncs{
				AddFunc:    tt.fields.AddFunc,
				UpdateFunc: tt.fields.UpdateFunc,
				DeleteFunc: tt.fields.DeleteFunc,
			}
			e.OnDelete(tt.args.table, tt.args.row)
			if e.DeleteFunc != nil {
				assert.Equal(t, 1, calls)
			}
		})
	}
}

func TestTableCacheTable(t *testing.T) {
	tests := []struct {
		name  string
		cache map[string]*RowCache
		table string
		want  *RowCache
	}{
		{
			"returns nil for an empty table",
			map[string]*RowCache{"bar": newRowCache("bar", ovsdb.TableSchema{}, nil)},
			"foo",
			nil,
		},
		{
			"returns nil for an empty table",
			map[string]*RowCache{"bar": newRowCache("bar", ovsdb.TableSchema{}, nil)},
			"bar",
			newRowCache("bar", ovsdb.TableSchema{}, nil),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &TableCache{
				cache: tt.cache,
			}
			got := tr.Table(tt.table)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTableCacheTables(t *testing.T) {
	tests := []struct {
		name  string
		cache map[string]*RowCache
		want  []string
	}{
		{
			"returns a table that exists",
			map[string]*RowCache{
				"test1": newRowCache("test1", ovsdb.TableSchema{}, nil),
				"test2": newRowCache("test2", ovsdb.TableSchema{}, nil),
				"test3": newRowCache("test3", ovsdb.TableSchema{}, nil),
			},
			[]string{"test1", "test2", "test3"},
		},
		{
			"returns an empty slice if no tables exist",
			map[string]*RowCache{},
			[]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &TableCache{
				cache: tt.cache,
			}
			got := tr.Tables()
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}

func TestTableCache_populate(t *testing.T) {
	t.Log("Create")
	db, err := model.NewDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	assert.Nil(t, err)
	var schema ovsdb.DatabaseSchema
	err = json.Unmarshal([]byte(`
		 {"name": "TestDB",
		  "tables": {
		    "Open_vSwitch": {
			  "indexes": [["foo"]],
		      "columns": {
		        "foo": {
			  "type": "string"
			},
			"bar": {
				"type": "string"
			  }
		      }
		    }
		 }
	     }
	`), &schema)
	assert.Nil(t, err)
	tc, err := NewTableCache(&schema, db, nil)
	assert.Nil(t, err)

	testRow := ovsdb.Row(map[string]interface{}{"_uuid": "test", "foo": "bar"})
	testRowModel := &testModel{UUID: "test", Foo: "bar"}
	updates := ovsdb.TableUpdates{
		"Open_vSwitch": {
			"test": &ovsdb.RowUpdate{
				Old: nil,
				New: &testRow,
			},
		},
	}
	tc.Populate(updates)

	got := tc.Table("Open_vSwitch").Row("test")
	assert.Equal(t, testRowModel, got)

	t.Log("Update")
	updatedRow := ovsdb.Row(map[string]interface{}{"_uuid": "test", "foo": "quux"})
	updatedRowModel := &testModel{UUID: "test", Foo: "quux"}
	updates["Open_vSwitch"]["test"] = &ovsdb.RowUpdate{
		Old: &testRow,
		New: &updatedRow,
	}
	tc.Populate(updates)

	got = tc.cache["Open_vSwitch"].cache["test"]
	assert.Equal(t, updatedRowModel, got)

	t.Log("Delete")
	updates["Open_vSwitch"]["test"] = &ovsdb.RowUpdate{
		Old: &updatedRow,
		New: nil,
	}

	tc.Populate(updates)

	_, ok := tc.cache["Open_vSwitch"].cache["test"]
	assert.False(t, ok)
}

func TestEventProcessor_AddEvent(t *testing.T) {
	ep := newEventProcessor(16)
	var events []event
	for i := 0; i < 17; i++ {
		events = append(events, event{
			table:     "bridge",
			eventType: addEvent,
			new: &testModel{
				UUID: "unique",
				Foo:  "bar",
			},
		})
	}
	// overfill channel so event 16 is dropped
	for _, e := range events {
		ep.AddEvent(e.eventType, e.table, nil, e.new)
	}
	// assert channel is full of events
	assert.Equal(t, 16, len(ep.events))

	// read events and ensure they are in FIFO order
	for i := 0; i < 16; i++ {
		event := <-ep.events
		assert.Equal(t, &testModel{UUID: "unique", Foo: "bar"}, event.new)
	}

	// assert channel is empty
	assert.Equal(t, 0, len(ep.events))
}

func TestIndex(t *testing.T) {
	type indexTestModel struct {
		UUID string `ovsdb:"_uuid"`
		Foo  string `ovsdb:"foo"`
		Bar  string `ovsdb:"bar"`
		Baz  int    `ovsdb:"baz"`
	}
	db, err := model.NewDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &indexTestModel{}})
	assert.Nil(t, err)
	var schema ovsdb.DatabaseSchema
	err = json.Unmarshal([]byte(`
		 {"name": "TestDB",
		  "tables": {
		    "Open_vSwitch": {
			  "indexes": [["foo"], ["bar","baz"]],
		      "columns": {
		        "foo": {
			  "type": "string"
			},
			"bar": {
				"type": "string"
			},
			"baz": {
				"type": "integer"
			}
		      }
		    }
		 }
	     }
	`), &schema)
	assert.Nil(t, err)
	tc, err := NewTableCache(&schema, db, nil)
	assert.Nil(t, err)
	obj := &indexTestModel{
		UUID: "test1",
		Foo:  "foo",
		Bar:  "bar",
		Baz:  42,
	}
	table := tc.Table("Open_vSwitch")

	err = table.Create(obj.UUID, obj, true)
	assert.Nil(t, err)
	t.Run("Index by single column", func(t *testing.T) {
		idx, err := table.Index("foo")
		assert.Nil(t, err)
		info, err := mapper.NewInfo(schema.Table("Open_vSwitch"), obj)
		assert.Nil(t, err)
		v, err := valueFromIndex(info, newIndex("foo"))
		assert.Nil(t, err)
		assert.Equal(t, idx[v], obj.UUID)
	})
	t.Run("Index by single column miss", func(t *testing.T) {
		idx, err := table.Index("foo")
		assert.Nil(t, err)
		obj2 := obj
		obj2.Foo = "wrong"
		assert.Nil(t, err)
		info, err := mapper.NewInfo(schema.Table("Open_vSwitch"), obj2)
		assert.Nil(t, err)
		v, err := valueFromIndex(info, newIndex("foo"))
		assert.Nil(t, err)
		_, ok := idx[v]
		assert.False(t, ok)
	})
	t.Run("Index by single column wrong", func(t *testing.T) {
		_, err := table.Index("wrong")
		assert.NotNil(t, err)
	})
	t.Run("Index by multi-column wrong", func(t *testing.T) {
		_, err := table.Index("bar", "wrong")
		assert.NotNil(t, err)
	})
	t.Run("Index by multi-column", func(t *testing.T) {
		idx, err := table.Index("bar", "baz")
		assert.Nil(t, err)
		info, err := mapper.NewInfo(schema.Table("Open_vSwitch"), obj)
		assert.Nil(t, err)
		v, err := valueFromIndex(info, newIndex("bar", "baz"))
		assert.Nil(t, err)
		assert.Equal(t, idx[v], obj.UUID)
	})
	t.Run("Index by multi-column miss", func(t *testing.T) {
		idx, err := table.Index("bar", "baz")
		assert.Nil(t, err)
		obj2 := obj
		obj2.Baz++
		info, err := mapper.NewInfo(schema.Table("Open_vSwitch"), obj)
		assert.Nil(t, err)
		v, err := valueFromIndex(info, newIndex("bar", "baz"))
		assert.Nil(t, err)
		_, ok := idx[v]
		assert.False(t, ok)
	})
	t.Run("Index type", func(t *testing.T) {
		idx := newIndex("foo", "bar")
		assert.Equal(t, idx.columns(), []string{"foo", "bar"})
	})
}

func TestTableCacheRowByModelSingleIndex(t *testing.T) {
	var schema ovsdb.DatabaseSchema
	db, err := model.NewDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	require.Nil(t, err)
	err = json.Unmarshal([]byte(`
		 {"name": "TestDB",
		  "tables": {
		    "Open_vSwitch": {
			  "indexes": [["foo"]],
		      "columns": {
		        "foo": {
			  "type": "string"
			},
			"bar": {
				"type": "string"
			  }
		      }
		    }
		 }
	     }
	`), &schema)
	require.NoError(t, err)
	myFoo := &testModel{Foo: "foo", Bar: "foo"}
	testData := Data{
		"Open_vSwitch": map[string]model.Model{
			"foo": myFoo,
			"bar": &testModel{Foo: "bar", Bar: "bar"},
		},
	}
	tc, err := NewTableCache(&schema, db, testData)
	require.NoError(t, err)

	t.Run("get foo by index", func(t *testing.T) {
		foo := tc.Table("Open_vSwitch").RowByModel(&testModel{Foo: "foo"})
		assert.NotNil(t, foo)
		assert.Equal(t, myFoo, foo)
	})

	t.Run("get non-existent item by index", func(t *testing.T) {
		baz := tc.Table("Open_vSwitch").RowByModel(&testModel{Foo: "baz"})
		assert.Nil(t, baz)
	})

	t.Run("no index data", func(t *testing.T) {
		foo := tc.Table("Open_vSwitch").RowByModel(&testModel{Bar: "foo"})
		assert.Nil(t, foo)
	})
}

func TestTableCacheRowByModelTwoIndexes(t *testing.T) {
	var schema ovsdb.DatabaseSchema
	db, err := model.NewDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	require.Nil(t, err)
	err = json.Unmarshal([]byte(`
		 {"name": "TestDB",
		  "tables": {
		    "Open_vSwitch": {
			  "indexes": [["foo"], ["bar"]],
		      "columns": {
		        "foo": {
			  "type": "string"
			},
			"bar": {
				"type": "string"
			  }
		      }
		    }
		 }
	     }
	`), &schema)
	require.NoError(t, err)
	myFoo := &testModel{Foo: "foo", Bar: "foo"}
	testData := Data{
		"Open_vSwitch": map[string]model.Model{
			"foo": myFoo,
			"bar": &testModel{Foo: "bar", Bar: "bar"},
		},
	}
	tc, err := NewTableCache(&schema, db, testData)
	require.NoError(t, err)

	t.Run("get foo by Foo index", func(t *testing.T) {
		foo := tc.Table("Open_vSwitch").RowByModel(&testModel{Foo: "foo"})
		assert.NotNil(t, foo)
		assert.Equal(t, myFoo, foo)
	})

	t.Run("get foo by Bar index", func(t *testing.T) {
		foo := tc.Table("Open_vSwitch").RowByModel(&testModel{Bar: "foo"})
		assert.NotNil(t, foo)
		assert.Equal(t, myFoo, foo)
	})

	t.Run("get non-existent item by index", func(t *testing.T) {
		baz := tc.Table("Open_vSwitch").RowByModel(&testModel{Foo: "baz"})
		assert.Nil(t, baz)
	})

}

func TestTableCacheRowByModelMultiIndex(t *testing.T) {
	var schema ovsdb.DatabaseSchema
	db, err := model.NewDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	require.Nil(t, err)
	err = json.Unmarshal([]byte(`
		 {"name": "TestDB",
		  "tables": {
		    "Open_vSwitch": {
			  "indexes": [["foo", "bar"]],
		      "columns": {
		        "foo": {
			  "type": "string"
			},
			"bar": {
				"type": "string"
			  }
		      }
		    }
		 }
	     }
	`), &schema)
	require.NoError(t, err)
	myFoo := &testModel{Foo: "foo", Bar: "foo"}
	testData := Data{
		"Open_vSwitch": map[string]model.Model{"foo": myFoo, "bar": &testModel{Foo: "bar", Bar: "bar"}},
	}
	tc, err := NewTableCache(&schema, db, testData)
	require.NoError(t, err)

	t.Run("incomplete index", func(t *testing.T) {
		foo := tc.Table("Open_vSwitch").RowByModel(&testModel{Foo: "foo"})
		assert.Nil(t, foo)
	})

	t.Run("get foo by index", func(t *testing.T) {
		foo := tc.Table("Open_vSwitch").RowByModel(&testModel{Foo: "foo", Bar: "foo"})
		assert.NotNil(t, foo)
		assert.Equal(t, myFoo, foo)
	})

	t.Run("get non-existent item by index", func(t *testing.T) {
		baz := tc.Table("Open_vSwitch").RowByModel(&testModel{Foo: "baz", Bar: "baz"})
		assert.Nil(t, baz)
	})
}
