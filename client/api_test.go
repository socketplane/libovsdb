package client

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ovn-org/libovsdb/cache"
	"github.com/ovn-org/libovsdb/model"
	"github.com/ovn-org/libovsdb/ovsdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	trueVal  = true
	falseVal = false
	one      = 1
	six      = 6
)

func TestAPIListSimple(t *testing.T) {

	lscacheList := []model.Model{
		&testLogicalSwitch{
			UUID:        aUUID0,
			Name:        "ls0",
			ExternalIds: map[string]string{"foo": "bar"},
		},
		&testLogicalSwitch{
			UUID:        aUUID1,
			Name:        "ls1",
			ExternalIds: map[string]string{"foo": "baz"},
		},
		&testLogicalSwitch{
			UUID:        aUUID2,
			Name:        "ls2",
			ExternalIds: map[string]string{"foo": "baz"},
		},
		&testLogicalSwitch{
			UUID:        aUUID3,
			Name:        "ls4",
			ExternalIds: map[string]string{"foo": "baz"},
			Ports:       []string{"port0", "port1"},
		},
	}
	lscache := map[string]model.Model{}
	for i := range lscacheList {
		lscache[lscacheList[i].(*testLogicalSwitch).UUID] = lscacheList[i]
	}
	testData := cache.Data{
		"Logical_Switch": lscache,
	}
	tcache := apiTestCache(t, testData)
	test := []struct {
		name       string
		initialCap int
		resultCap  int
		resultLen  int
		content    []model.Model
		err        bool
	}{
		{
			name:       "full",
			initialCap: 0,
			resultCap:  len(lscache),
			resultLen:  len(lscacheList),
			content:    lscacheList,
			err:        false,
		},
		{
			name:       "single",
			initialCap: 1,
			resultCap:  1,
			resultLen:  1,
			content:    lscacheList[0:0],
			err:        false,
		},
		{
			name:       "multiple",
			initialCap: 2,
			resultCap:  2,
			resultLen:  2,
			content:    lscacheList[0:2],
			err:        false,
		},
	}
	for _, tt := range test {
		t.Run(fmt.Sprintf("ApiList: %s", tt.name), func(t *testing.T) {
			var result []testLogicalSwitch
			if tt.initialCap != 0 {
				result = make([]testLogicalSwitch, tt.initialCap)
			}
			api := newAPI(tcache)
			err := api.List(&result)
			if tt.err {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.Lenf(t, result, tt.resultLen, "Length should match expected")
				assert.Equal(t, cap(result), tt.resultCap, "Capability should match expected")
				assert.ElementsMatchf(t, tt.content, tt.content, "Content should match")
			}

		})
	}

	t.Run("ApiList: Error wrong type", func(t *testing.T) {
		var result []string
		api := newAPI(tcache)
		err := api.List(&result)
		assert.NotNil(t, err)
	})

	t.Run("ApiList: Type Selection", func(t *testing.T) {
		var result []testLogicalSwitchPort
		api := newAPI(tcache)
		err := api.List(&result)
		assert.Nil(t, err)
		assert.Len(t, result, 0, "Should be empty since cache is empty")
	})

	t.Run("ApiList: Empty List", func(t *testing.T) {
		result := []testLogicalSwitch{}
		api := newAPI(tcache)
		err := api.List(&result)
		assert.Nil(t, err)
		assert.Len(t, result, len(lscacheList))
	})
}

func TestAPIListPredicate(t *testing.T) {
	lscacheList := []model.Model{
		&testLogicalSwitch{
			UUID:        aUUID0,
			Name:        "ls0",
			ExternalIds: map[string]string{"foo": "bar"},
		},
		&testLogicalSwitch{
			UUID:        aUUID1,
			Name:        "magicLs1",
			ExternalIds: map[string]string{"foo": "baz"},
		},
		&testLogicalSwitch{
			UUID:        aUUID2,
			Name:        "ls2",
			ExternalIds: map[string]string{"foo": "baz"},
		},
		&testLogicalSwitch{
			UUID:        aUUID3,
			Name:        "magicLs2",
			ExternalIds: map[string]string{"foo": "baz"},
			Ports:       []string{"port0", "port1"},
		},
	}
	lscache := map[string]model.Model{}
	for i := range lscacheList {
		lscache[lscacheList[i].(*testLogicalSwitch).UUID] = lscacheList[i]
	}
	testData := cache.Data{
		"Logical_Switch": lscache,
	}
	tcache := apiTestCache(t, testData)

	test := []struct {
		name      string
		predicate interface{}
		content   []model.Model
		err       bool
	}{
		{
			name: "none",
			predicate: func(t *testLogicalSwitch) bool {
				return false
			},
			content: []model.Model{},
			err:     false,
		},
		{
			name: "all",
			predicate: func(t *testLogicalSwitch) bool {
				return true
			},
			content: lscacheList,
			err:     false,
		},
		{
			name: "nil function must fail",
			err:  true,
		},
		{
			name: "arbitrary condition",
			predicate: func(t *testLogicalSwitch) bool {
				return strings.HasPrefix(t.Name, "magic")
			},
			content: []model.Model{lscacheList[1], lscacheList[3]},
			err:     false,
		},
		{
			name: "error wrong type",
			predicate: func(t testLogicalSwitch) string {
				return "foo"
			},
			err: true,
		},
	}

	for _, tt := range test {
		t.Run(fmt.Sprintf("ApiListPredicate: %s", tt.name), func(t *testing.T) {
			var result []testLogicalSwitch
			api := newAPI(tcache)
			cond := api.WhereCache(tt.predicate)
			err := cond.List(&result)
			if tt.err {
				assert.NotNil(t, err)
			} else {
				if !assert.Nil(t, err) {
					t.Log(err)
				}
				assert.ElementsMatchf(t, tt.content, tt.content, "Content should match")
			}

		})
	}
}

func TestAPIListFields(t *testing.T) {
	lspcacheList := []model.Model{
		&testLogicalSwitchPort{
			UUID:        aUUID0,
			Name:        "lsp0",
			ExternalIds: map[string]string{"foo": "bar"},
			Enabled:     &trueVal,
		},
		&testLogicalSwitchPort{
			UUID:        aUUID1,
			Name:        "magiclsp1",
			ExternalIds: map[string]string{"foo": "baz"},
			Enabled:     &falseVal,
		},
		&testLogicalSwitchPort{
			UUID:        aUUID2,
			Name:        "lsp2",
			ExternalIds: map[string]string{"unique": "id"},
			Enabled:     &falseVal,
		},
		&testLogicalSwitchPort{
			UUID:        aUUID3,
			Name:        "magiclsp2",
			ExternalIds: map[string]string{"foo": "baz"},
			Enabled:     &trueVal,
		},
	}
	lspcache := map[string]model.Model{}
	for i := range lspcacheList {
		lspcache[lspcacheList[i].(*testLogicalSwitchPort).UUID] = lspcacheList[i]
	}
	testData := cache.Data{
		"Logical_Switch_Port": lspcache,
	}
	tcache := apiTestCache(t, testData)

	testObj := testLogicalSwitchPort{}

	test := []struct {
		name    string
		fields  []interface{}
		prepare func(*testLogicalSwitchPort)
		content []model.Model
		err     bool
	}{
		{
			name:    "empty object must match everything",
			content: lspcacheList,
			err:     false,
		},
		{
			name: "List unique by UUID",
			prepare: func(t *testLogicalSwitchPort) {
				t.UUID = aUUID0
			},
			content: []model.Model{lspcache[aUUID0]},
			err:     false,
		},
		{
			name: "List unique by Index",
			prepare: func(t *testLogicalSwitchPort) {
				t.Name = "lsp2"
			},
			content: []model.Model{lspcache[aUUID2]},
			err:     false,
		},
	}

	for _, tt := range test {
		t.Run(fmt.Sprintf("ApiListFields: %s", tt.name), func(t *testing.T) {
			var result []testLogicalSwitchPort
			// Clean object
			testObj = testLogicalSwitchPort{}
			api := newAPI(tcache)
			err := api.Where(&testObj).List(&result)
			if tt.err {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.ElementsMatchf(t, tt.content, tt.content, "Content should match")
			}

		})
	}

	t.Run("ApiListFields: Wrong table", func(t *testing.T) {
		var result []testLogicalSwitchPort
		api := newAPI(tcache)
		obj := testLogicalSwitch{
			UUID: aUUID0,
		}

		err := api.Where(&obj).List(&result)
		assert.NotNil(t, err)
	})
}

func TestConditionFromFunc(t *testing.T) {
	test := []struct {
		name string
		arg  interface{}
		err  bool
	}{
		{
			name: "wrong function must fail",
			arg: func(s string) bool {
				return false
			},
			err: true,
		},
		{
			name: "wrong function must fail2 ",
			arg: func(t *testLogicalSwitch) string {
				return "foo"
			},
			err: true,
		},
		{
			name: "correct func should succeed",
			arg: func(t *testLogicalSwitch) bool {
				return true
			},
			err: false,
		},
	}

	for _, tt := range test {
		t.Run(fmt.Sprintf("conditionFromFunc: %s", tt.name), func(t *testing.T) {
			cache := apiTestCache(t, nil)
			apiIface := newAPI(cache)
			condition := apiIface.(api).conditionFromFunc(tt.arg)
			if tt.err {
				assert.IsType(t, &errorConditional{}, condition)
			} else {
				assert.IsType(t, &predicateConditional{}, condition)
			}
		})
	}
}

func TestConditionFromModel(t *testing.T) {
	var testObj testLogicalSwitch
	test := []struct {
		name  string
		model model.Model
		conds []model.Condition
		err   bool
	}{
		{
			name:  "wrong model must fail",
			model: &struct{ a string }{},
			err:   true,
		},
		{
			name: "wrong condition must fail",
			model: &struct {
				a string `ovsdb:"_uuid"`
			}{},
			conds: []model.Condition{{Field: "foo"}},
			err:   true,
		},
		{
			name:  "correct model must succeed",
			model: &testLogicalSwitch{},
			err:   false,
		},
		{
			name:  "correct model with valid condition must succeed",
			model: &testObj,
			conds: []model.Condition{
				{
					Field:    &testObj.Name,
					Function: ovsdb.ConditionEqual,
					Value:    "foo",
				},
				{
					Field:    &testObj.Ports,
					Function: ovsdb.ConditionIncludes,
					Value:    []string{"foo"},
				},
			},
			err: false,
		},
	}

	for _, tt := range test {
		t.Run(fmt.Sprintf("conditionFromModel: %s", tt.name), func(t *testing.T) {
			cache := apiTestCache(t, nil)
			apiIface := newAPI(cache)
			condition := apiIface.(api).conditionFromModel(false, tt.model, tt.conds...)
			if tt.err {
				assert.IsType(t, &errorConditional{}, condition)
			} else {
				if len(tt.conds) > 0 {
					assert.IsType(t, &explicitConditional{}, condition)
				} else {
					assert.IsType(t, &equalityConditional{}, condition)
				}

			}
		})
	}
}

func TestAPIGet(t *testing.T) {
	lsCacheList := []model.Model{}
	lspCacheList := []model.Model{
		&testLogicalSwitchPort{
			UUID:        aUUID2,
			Name:        "lsp0",
			Type:        "foo",
			ExternalIds: map[string]string{"foo": "bar"},
		},
		&testLogicalSwitchPort{
			UUID:        aUUID3,
			Name:        "lsp1",
			Type:        "bar",
			ExternalIds: map[string]string{"foo": "baz"},
		},
	}
	lsCache := map[string]model.Model{}
	lspCache := map[string]model.Model{}
	for i := range lsCacheList {
		lsCache[lsCacheList[i].(*testLogicalSwitch).UUID] = lsCacheList[i]
	}
	for i := range lspCacheList {
		lspCache[lspCacheList[i].(*testLogicalSwitchPort).UUID] = lspCacheList[i]
	}
	testData := cache.Data{
		"Logical_Switch":      lsCache,
		"Logical_Switch_Port": lspCache,
	}
	tcache := apiTestCache(t, testData)

	test := []struct {
		name    string
		prepare func(model.Model)
		result  model.Model
		err     bool
	}{
		{
			name: "empty",
			prepare: func(m model.Model) {
			},
			err: true,
		},
		{
			name: "non_existing",
			prepare: func(m model.Model) {
				m.(*testLogicalSwitchPort).Name = "foo"
			},
			err: true,
		},
		{
			name: "by UUID",
			prepare: func(m model.Model) {
				m.(*testLogicalSwitchPort).UUID = aUUID3
			},
			result: lspCacheList[1],
			err:    false,
		},
		{
			name: "by name",
			prepare: func(m model.Model) {
				m.(*testLogicalSwitchPort).Name = "lsp0"
			},
			result: lspCacheList[0],
			err:    false,
		},
	}
	for _, tt := range test {
		t.Run(fmt.Sprintf("ApiGet: %s", tt.name), func(t *testing.T) {
			var result testLogicalSwitchPort
			tt.prepare(&result)
			api := newAPI(tcache)
			err := api.Get(&result)
			if tt.err {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.Equalf(t, tt.result, &result, "Result should match")
			}
		})
	}
}

func TestAPICreate(t *testing.T) {
	lsCacheList := []model.Model{}
	lspCacheList := []model.Model{
		&testLogicalSwitchPort{
			UUID:        aUUID2,
			Name:        "lsp0",
			Type:        "foo",
			ExternalIds: map[string]string{"foo": "bar"},
		},
		&testLogicalSwitchPort{
			UUID:        aUUID3,
			Name:        "lsp1",
			Type:        "bar",
			ExternalIds: map[string]string{"foo": "baz"},
		},
	}
	lsCache := map[string]model.Model{}
	lspCache := map[string]model.Model{}
	for i := range lsCacheList {
		lsCache[lsCacheList[i].(*testLogicalSwitch).UUID] = lsCacheList[i]
	}
	for i := range lspCacheList {
		lspCache[lspCacheList[i].(*testLogicalSwitchPort).UUID] = lspCacheList[i]
	}
	testData := cache.Data{
		"Logical_Switch":      lsCache,
		"Logical_Switch_Port": lspCache,
	}
	tcache := apiTestCache(t, testData)

	rowFoo := ovsdb.Row(map[string]interface{}{"name": "foo"})
	rowBar := ovsdb.Row(map[string]interface{}{"name": "bar"})
	test := []struct {
		name   string
		input  []model.Model
		result []ovsdb.Operation
		err    bool
	}{
		{
			name:  "empty",
			input: []model.Model{&testLogicalSwitch{}},
			result: []ovsdb.Operation{{
				Op:       "insert",
				Table:    "Logical_Switch",
				Row:      ovsdb.Row{},
				UUIDName: "",
			}},
			err: false,
		},
		{
			name: "With some values",
			input: []model.Model{&testLogicalSwitch{
				Name: "foo",
			}},
			result: []ovsdb.Operation{{
				Op:       "insert",
				Table:    "Logical_Switch",
				Row:      rowFoo,
				UUIDName: "",
			}},
			err: false,
		},
		{
			name: "With named UUID ",
			input: []model.Model{&testLogicalSwitch{
				UUID: "foo",
			}},
			result: []ovsdb.Operation{{
				Op:       "insert",
				Table:    "Logical_Switch",
				Row:      ovsdb.Row{},
				UUIDName: "foo",
			}},
			err: false,
		},
		{
			name: "Multiple",
			input: []model.Model{
				&testLogicalSwitch{
					UUID: "fooUUID",
					Name: "foo",
				},
				&testLogicalSwitch{
					UUID: "barUUID",
					Name: "bar",
				},
			},
			result: []ovsdb.Operation{{
				Op:       "insert",
				Table:    "Logical_Switch",
				Row:      rowFoo,
				UUIDName: "fooUUID",
			}, {
				Op:       "insert",
				Table:    "Logical_Switch",
				Row:      rowBar,
				UUIDName: "barUUID",
			}},
			err: false,
		},
	}
	for _, tt := range test {
		t.Run(fmt.Sprintf("ApiCreate: %s", tt.name), func(t *testing.T) {
			api := newAPI(tcache)
			op, err := api.Create(tt.input...)
			if tt.err {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.Equalf(t, tt.result, op, "ovsdb.Operation should match")
			}
		})
	}
}

func TestAPIMutate(t *testing.T) {
	lspCache := map[string]model.Model{
		aUUID0: &testLogicalSwitchPort{
			UUID:        aUUID0,
			Name:        "lsp0",
			Type:        "someType",
			ExternalIds: map[string]string{"foo": "bar"},
			Enabled:     &trueVal,
			Tag:         &one,
		},
		aUUID1: &testLogicalSwitchPort{
			UUID:        aUUID1,
			Name:        "lsp1",
			Type:        "someType",
			ExternalIds: map[string]string{"foo": "baz"},
			Tag:         &one,
		},
		aUUID2: &testLogicalSwitchPort{
			UUID:        aUUID2,
			Name:        "lsp2",
			Type:        "someOtherType",
			ExternalIds: map[string]string{"foo": "baz"},
			Tag:         &one,
		},
	}
	testData := cache.Data{
		"Logical_Switch_Port": lspCache,
	}
	tcache := apiTestCache(t, testData)

	testObj := testLogicalSwitchPort{}
	test := []struct {
		name      string
		condition func(API) ConditionalAPI
		model     model.Model
		mutations []model.Mutation
		init      map[string]model.Model
		result    []ovsdb.Operation
		err       bool
	}{
		{
			name: "select by UUID addElement to set",
			condition: func(a API) ConditionalAPI {
				return a.Where(&testLogicalSwitch{
					UUID: aUUID0,
				})
			},
			mutations: []model.Mutation{
				{
					Field:   &testObj.Addresses,
					Mutator: ovsdb.MutateOperationInsert,
					Value:   []string{"1.1.1.1"},
				},
			},
			result: []ovsdb.Operation{
				{
					Op:        ovsdb.OperationMutate,
					Table:     "Logical_Switch_Port",
					Mutations: []ovsdb.Mutation{{Column: "addresses", Mutator: ovsdb.MutateOperationInsert, Value: testOvsSet(t, []string{"1.1.1.1"})}},
					Where:     []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID0}}},
				},
			},
			err: false,
		},
		{
			name: "select by name delete element from map",
			condition: func(a API) ConditionalAPI {
				return a.Where(&testLogicalSwitchPort{
					Name: "lsp2",
				})
			},
			mutations: []model.Mutation{
				{
					Field:   &testObj.ExternalIds,
					Mutator: ovsdb.MutateOperationDelete,
					Value:   []string{"foo"},
				},
			},
			result: []ovsdb.Operation{
				{
					Op:        ovsdb.OperationMutate,
					Table:     "Logical_Switch_Port",
					Mutations: []ovsdb.Mutation{{Column: "external_ids", Mutator: ovsdb.MutateOperationDelete, Value: testOvsSet(t, []string{"foo"})}},
					Where:     []ovsdb.Condition{{Column: "name", Function: ovsdb.ConditionEqual, Value: "lsp2"}},
				},
			},
			err: false,
		},
		{
			name: "select single by predicate name insert element in map",
			condition: func(a API) ConditionalAPI {
				return a.WhereCache(func(lsp *testLogicalSwitchPort) bool {
					return lsp.Name == "lsp2"
				})
			},
			mutations: []model.Mutation{
				{
					Field:   &testObj.ExternalIds,
					Mutator: ovsdb.MutateOperationInsert,
					Value:   map[string]string{"bar": "baz"},
				},
			},
			result: []ovsdb.Operation{
				{
					Op:        ovsdb.OperationMutate,
					Table:     "Logical_Switch_Port",
					Mutations: []ovsdb.Mutation{{Column: "external_ids", Mutator: ovsdb.MutateOperationInsert, Value: testOvsMap(t, map[string]string{"bar": "baz"})}},
					Where:     []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID2}}},
				},
			},
			err: false,
		},
		{
			name: "select many by predicate name insert element in map",
			condition: func(a API) ConditionalAPI {
				return a.WhereCache(func(lsp *testLogicalSwitchPort) bool {
					return lsp.Type == "someType"
				})
			},
			mutations: []model.Mutation{
				{
					Field:   &testObj.ExternalIds,
					Mutator: ovsdb.MutateOperationInsert,
					Value:   map[string]string{"bar": "baz"},
				},
			},
			result: []ovsdb.Operation{
				{
					Op:        ovsdb.OperationMutate,
					Table:     "Logical_Switch_Port",
					Mutations: []ovsdb.Mutation{{Column: "external_ids", Mutator: ovsdb.MutateOperationInsert, Value: testOvsMap(t, map[string]string{"bar": "baz"})}},
					Where:     []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID0}}},
				},
				{
					Op:        ovsdb.OperationMutate,
					Table:     "Logical_Switch_Port",
					Mutations: []ovsdb.Mutation{{Column: "external_ids", Mutator: ovsdb.MutateOperationInsert, Value: testOvsMap(t, map[string]string{"bar": "baz"})}},
					Where:     []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID1}}},
				},
			},
			err: false,
		},
		{
			name: "No mutations should error",
			condition: func(a API) ConditionalAPI {
				return a.WhereCache(func(lsp *testLogicalSwitchPort) bool {
					return lsp.Type == "someType"
				})
			},
			mutations: []model.Mutation{},
			err:       true,
		},
	}
	for _, tt := range test {
		t.Run(fmt.Sprintf("ApiMutate: %s", tt.name), func(t *testing.T) {
			api := newAPI(tcache)
			cond := tt.condition(api)
			ops, err := cond.Mutate(&testObj, tt.mutations...)
			if tt.err {
				require.Error(t, err)
			} else {
				require.Nil(t, err)
				assert.ElementsMatchf(t, tt.result, ops, "ovsdb.Operations should match")
			}
		})
	}
}

func TestAPIUpdate(t *testing.T) {
	lspCache := map[string]model.Model{
		aUUID0: &testLogicalSwitchPort{
			UUID:        aUUID0,
			Name:        "lsp0",
			Type:        "someType",
			ExternalIds: map[string]string{"foo": "bar"},
			Enabled:     &trueVal,
			Tag:         &one,
		},
		aUUID1: &testLogicalSwitchPort{
			UUID:        aUUID1,
			Name:        "lsp1",
			Type:        "someType",
			ExternalIds: map[string]string{"foo": "baz"},
			Tag:         &one,
			Enabled:     &trueVal,
		},
		aUUID2: &testLogicalSwitchPort{
			UUID:        aUUID2,
			Name:        "lsp2",
			Type:        "someOtherType",
			ExternalIds: map[string]string{"foo": "baz"},
			Tag:         &one,
		},
	}
	testData := cache.Data{
		"Logical_Switch_Port": lspCache,
	}
	tcache := apiTestCache(t, testData)

	testObj := testLogicalSwitchPort{}
	testRow := ovsdb.Row(map[string]interface{}{"type": "somethingElse", "tag": testOvsSet(t, []int{6})})
	tagRow := ovsdb.Row(map[string]interface{}{"tag": testOvsSet(t, []int{6})})
	test := []struct {
		name      string
		condition func(API) ConditionalAPI
		prepare   func(t *testLogicalSwitchPort)
		result    []ovsdb.Operation
		err       bool
	}{
		{
			name: "select by UUID change multiple field",
			condition: func(a API) ConditionalAPI {
				return a.Where(&testLogicalSwitch{
					UUID: aUUID0,
				})
			},
			prepare: func(t *testLogicalSwitchPort) {
				t.Type = "somethingElse"
				t.Tag = &six
			},
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationUpdate,
					Table: "Logical_Switch_Port",
					Row:   testRow,
					Where: []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID0}}},
				},
			},
			err: false,
		},
		{
			name: "select by index change multiple field",
			condition: func(a API) ConditionalAPI {
				return a.Where(&testLogicalSwitchPort{
					Name: "lsp1",
				})
			},
			prepare: func(t *testLogicalSwitchPort) {
				t.Type = "somethingElse"
				t.Tag = &six
			},
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationUpdate,
					Table: "Logical_Switch_Port",
					Row:   testRow,
					Where: []ovsdb.Condition{{Column: "name", Function: ovsdb.ConditionEqual, Value: "lsp1"}},
				},
			},
			err: false,
		},
		{
			name: "select by field change multiple field",
			condition: func(a API) ConditionalAPI {
				t := testLogicalSwitchPort{
					Type:    "sometype",
					Enabled: &trueVal,
				}
				return a.Where(&t, model.Condition{
					Field:    &t.Type,
					Function: ovsdb.ConditionEqual,
					Value:    "sometype",
				})
			},
			prepare: func(t *testLogicalSwitchPort) {
				t.Tag = &six
			},
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationUpdate,
					Table: "Logical_Switch_Port",
					Row:   tagRow,
					Where: []ovsdb.Condition{{Column: "type", Function: ovsdb.ConditionEqual, Value: "sometype"}},
				},
			},
			err: false,
		},
		{
			name: "multiple select any by field change multiple field",
			condition: func(a API) ConditionalAPI {
				t := testLogicalSwitchPort{}
				return a.Where(&t,
					model.Condition{
						Field:    &t.Type,
						Function: ovsdb.ConditionEqual,
						Value:    "sometype",
					},
					model.Condition{
						Field:    &t.Enabled,
						Function: ovsdb.ConditionIncludes,
						Value:    &trueVal,
					})
			},
			prepare: func(t *testLogicalSwitchPort) {
				t.Tag = &six
			},
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationUpdate,
					Table: "Logical_Switch_Port",
					Row:   tagRow,
					Where: []ovsdb.Condition{{Column: "type", Function: ovsdb.ConditionEqual, Value: "sometype"}},
				},
				{
					Op:    ovsdb.OperationUpdate,
					Table: "Logical_Switch_Port",
					Row:   tagRow,
					Where: []ovsdb.Condition{{Column: "enabled", Function: ovsdb.ConditionIncludes, Value: testOvsSet(t, &trueVal)}},
				},
			},
			err: false,
		},
		{
			name: "multiple select all by field change multiple field",
			condition: func(a API) ConditionalAPI {
				t := testLogicalSwitchPort{}
				return a.WhereAll(&t,
					model.Condition{
						Field:    &t.Type,
						Function: ovsdb.ConditionEqual,
						Value:    "sometype",
					},
					model.Condition{
						Field:    &t.Enabled,
						Function: ovsdb.ConditionIncludes,
						Value:    &trueVal,
					})
			},
			prepare: func(t *testLogicalSwitchPort) {
				t.Tag = &six
			},
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationUpdate,
					Table: "Logical_Switch_Port",
					Row:   tagRow,
					Where: []ovsdb.Condition{
						{Column: "type", Function: ovsdb.ConditionEqual, Value: "sometype"},
						{Column: "enabled", Function: ovsdb.ConditionIncludes, Value: testOvsSet(t, &trueVal)},
					},
				},
			},
			err: false,
		},
		{
			name: "select by field inequality change multiple field",
			condition: func(a API) ConditionalAPI {
				t := testLogicalSwitchPort{
					Type:    "sometype",
					Enabled: &trueVal,
				}
				return a.Where(&t, model.Condition{
					Field:    &t.Type,
					Function: ovsdb.ConditionNotEqual,
					Value:    "sometype",
				})
			},
			prepare: func(t *testLogicalSwitchPort) {
				t.Tag = &six
			},
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationUpdate,
					Table: "Logical_Switch_Port",
					Row:   tagRow,
					Where: []ovsdb.Condition{{Column: "type", Function: ovsdb.ConditionNotEqual, Value: "sometype"}},
				},
			},
			err: false,
		},
		{
			name: "select multiple by predicate change multiple field",
			condition: func(a API) ConditionalAPI {
				return a.WhereCache(func(t *testLogicalSwitchPort) bool {
					return t.Enabled != nil && *t.Enabled == true
				})
			},
			prepare: func(t *testLogicalSwitchPort) {
				t.Type = "somethingElse"
				t.Tag = &six
			},
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationUpdate,
					Table: "Logical_Switch_Port",
					Row:   testRow,
					Where: []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID0}}},
				},
				{
					Op:    ovsdb.OperationUpdate,
					Table: "Logical_Switch_Port",
					Row:   testRow,
					Where: []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID1}}},
				},
			},
			err: false,
		},
	}
	for _, tt := range test {
		t.Run(fmt.Sprintf("ApiUpdate: %s", tt.name), func(t *testing.T) {
			api := newAPI(tcache)
			cond := tt.condition(api)
			// clean test Object
			testObj = testLogicalSwitchPort{}
			tt.prepare(&testObj)
			ops, err := cond.Update(&testObj)
			if tt.err {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.ElementsMatchf(t, tt.result, ops, "ovsdb.Operations should match")
			}
		})
	}
}

func TestAPIDelete(t *testing.T) {
	lspCache := map[string]model.Model{
		aUUID0: &testLogicalSwitchPort{
			UUID:        aUUID0,
			Name:        "lsp0",
			Type:        "someType",
			ExternalIds: map[string]string{"foo": "bar"},
			Enabled:     &trueVal,
			Tag:         &one,
		},
		aUUID1: &testLogicalSwitchPort{
			UUID:        aUUID1,
			Name:        "lsp1",
			Type:        "someType",
			ExternalIds: map[string]string{"foo": "baz"},
			Tag:         &one,
			Enabled:     &trueVal,
		},
		aUUID2: &testLogicalSwitchPort{
			UUID:        aUUID2,
			Name:        "lsp2",
			Type:        "someOtherType",
			ExternalIds: map[string]string{"foo": "baz"},
			Tag:         &one,
		},
	}
	testData := cache.Data{
		"Logical_Switch_Port": lspCache,
	}
	tcache := apiTestCache(t, testData)

	test := []struct {
		name      string
		condition func(API) ConditionalAPI
		result    []ovsdb.Operation
		err       bool
	}{
		{
			name: "select by UUID",
			condition: func(a API) ConditionalAPI {
				return a.Where(&testLogicalSwitch{
					UUID: aUUID0,
				})
			},
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationDelete,
					Table: "Logical_Switch",
					Where: []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID0}}},
				},
			},
			err: false,
		},
		{
			name: "select by index",
			condition: func(a API) ConditionalAPI {
				return a.Where(&testLogicalSwitchPort{
					Name: "lsp1",
				})
			},
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationDelete,
					Table: "Logical_Switch_Port",
					Where: []ovsdb.Condition{{Column: "name", Function: ovsdb.ConditionEqual, Value: "lsp1"}},
				},
			},
			err: false,
		},
		{
			name: "select by field equality",
			condition: func(a API) ConditionalAPI {
				t := testLogicalSwitchPort{
					Enabled: &trueVal,
				}
				return a.Where(&t, model.Condition{
					Field:    &t.Type,
					Function: ovsdb.ConditionEqual,
					Value:    "sometype",
				})
			},
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationDelete,
					Table: "Logical_Switch_Port",
					Where: []ovsdb.Condition{{Column: "type", Function: ovsdb.ConditionEqual, Value: "sometype"}},
				},
			},
			err: false,
		},
		{
			name: "select any by field ",
			condition: func(a API) ConditionalAPI {
				t := testLogicalSwitchPort{
					Enabled: &trueVal,
				}
				return a.Where(&t,
					model.Condition{
						Field:    &t.Type,
						Function: ovsdb.ConditionEqual,
						Value:    "sometype",
					}, model.Condition{
						Field:    &t.Name,
						Function: ovsdb.ConditionEqual,
						Value:    "foo",
					})
			},
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationDelete,
					Table: "Logical_Switch_Port",
					Where: []ovsdb.Condition{{Column: "type", Function: ovsdb.ConditionEqual, Value: "sometype"}},
				},
				{
					Op:    ovsdb.OperationDelete,
					Table: "Logical_Switch_Port",
					Where: []ovsdb.Condition{{Column: "name", Function: ovsdb.ConditionEqual, Value: "foo"}},
				},
			},
			err: false,
		},
		{
			name: "select all by field ",
			condition: func(a API) ConditionalAPI {
				t := testLogicalSwitchPort{
					Enabled: &trueVal,
				}
				return a.WhereAll(&t,
					model.Condition{
						Field:    &t.Type,
						Function: ovsdb.ConditionEqual,
						Value:    "sometype",
					}, model.Condition{
						Field:    &t.Name,
						Function: ovsdb.ConditionEqual,
						Value:    "foo",
					})
			},
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationDelete,
					Table: "Logical_Switch_Port",
					Where: []ovsdb.Condition{
						{Column: "type", Function: ovsdb.ConditionEqual, Value: "sometype"},
						{Column: "name", Function: ovsdb.ConditionEqual, Value: "foo"},
					},
				},
			},
			err: false,
		},
		{
			name: "select multiple by predicate",
			condition: func(a API) ConditionalAPI {
				return a.WhereCache(func(t *testLogicalSwitchPort) bool {
					return t.Enabled != nil && *t.Enabled == true
				})
			},
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationDelete,
					Table: "Logical_Switch_Port",
					Where: []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID0}}},
				},
				{
					Op:    ovsdb.OperationDelete,
					Table: "Logical_Switch_Port",
					Where: []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID1}}},
				},
			},
			err: false,
		},
	}
	for _, tt := range test {
		t.Run(fmt.Sprintf("ApiDelete: %s", tt.name), func(t *testing.T) {
			api := newAPI(tcache)
			cond := tt.condition(api)
			ops, err := cond.Delete()
			if tt.err {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.ElementsMatchf(t, tt.result, ops, "ovsdb.Operations should match")
			}
		})
	}
}
