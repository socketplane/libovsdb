package client

import (
	"fmt"
	"testing"

	"github.com/ovn-org/libovsdb/cache"
	"github.com/ovn-org/libovsdb/model"
	"github.com/ovn-org/libovsdb/ovsdb"
	"github.com/stretchr/testify/assert"
)

func TestEqualityConditional(t *testing.T) {
	lspcacheList := []model.Model{
		&testLogicalSwitchPort{
			UUID:        aUUID0,
			Name:        "lsp0",
			ExternalIds: map[string]string{"foo": "bar"},
			Enabled:     &trueVal,
		},
		&testLogicalSwitchPort{
			UUID:        aUUID1,
			Name:        "lsp1",
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
			Name:        "lsp3",
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

	test := []struct {
		name      string
		model     model.Model
		condition [][]ovsdb.Condition
		matches   map[model.Model]bool
		all       bool
		err       bool
	}{
		{
			name:  "by uuid",
			model: &testLogicalSwitchPort{UUID: aUUID0, Name: "different"},
			condition: [][]ovsdb.Condition{
				{
					{
						Column:   "_uuid",
						Function: ovsdb.ConditionEqual,
						Value:    ovsdb.UUID{GoUUID: aUUID0},
					}}},
			matches: map[model.Model]bool{
				&testLogicalSwitchPort{UUID: aUUID0}:              true,
				&testLogicalSwitchPort{UUID: aUUID1}:              false,
				&testLogicalSwitchPort{UUID: aUUID0, Name: "foo"}: true,
			},
		},
		{
			name:  "by uuid all",
			model: &testLogicalSwitchPort{UUID: aUUID0, Name: "different"},
			condition: [][]ovsdb.Condition{
				{
					{
						Column:   "_uuid",
						Function: ovsdb.ConditionEqual,
						Value:    ovsdb.UUID{GoUUID: aUUID0},
					}}},
			matches: map[model.Model]bool{
				&testLogicalSwitchPort{UUID: aUUID0}:              true,
				&testLogicalSwitchPort{UUID: aUUID1}:              false,
				&testLogicalSwitchPort{UUID: aUUID0, Name: "foo"}: true,
			},
			all: true,
		},
		{
			name:  "by index",
			model: &testLogicalSwitchPort{Name: "lsp1"},
			condition: [][]ovsdb.Condition{
				{
					{
						Column:   "name",
						Function: ovsdb.ConditionEqual,
						Value:    "lsp1",
					}}},
			matches: map[model.Model]bool{
				&testLogicalSwitchPort{UUID: aUUID1}:               false,
				&testLogicalSwitchPort{UUID: aUUID1, Name: "lsp1"}: true,
				&testLogicalSwitchPort{UUID: aUUID0, Name: "lsp1"}: true,
			},
		},
		{
			name:  "by index all",
			model: &testLogicalSwitchPort{Name: "lsp1"},
			condition: [][]ovsdb.Condition{
				{
					{
						Column:   "name",
						Function: ovsdb.ConditionEqual,
						Value:    "lsp1",
					}}},
			matches: map[model.Model]bool{
				&testLogicalSwitchPort{UUID: aUUID1}:               false,
				&testLogicalSwitchPort{UUID: aUUID1, Name: "lsp1"}: true,
				&testLogicalSwitchPort{UUID: aUUID0, Name: "lsp1"}: true,
			},
			all: true,
		},
		{
			name:  "by non index",
			model: &testLogicalSwitchPort{ExternalIds: map[string]string{"foo": "baz"}},
			err:   true,
		},
	}
	for _, tt := range test {
		t.Run(fmt.Sprintf("Equality Conditional: %s", tt.name), func(t *testing.T) {
			cond, err := newEqualityConditional(tcache.Mapper(), "Logical_Switch_Port", tt.all, tt.model)
			assert.Nil(t, err)
			for model, shouldMatch := range tt.matches {
				matches, err := cond.Matches(model)
				if tt.err {
					assert.NotNil(t, err)
				} else {
					assert.Nil(t, err)
					assert.Equalf(t, shouldMatch, matches, fmt.Sprintf("Match on model %#+v should be %v", model, shouldMatch))
				}
			}
			generated, err := cond.Generate()
			if tt.err {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.ElementsMatch(t, tt.condition, generated)
			}
		})
	}
}

func TestPredicateConditional(t *testing.T) {
	lspcacheList := []model.Model{
		&testLogicalSwitchPort{
			UUID:        aUUID0,
			Name:        "lsp0",
			ExternalIds: map[string]string{"foo": "bar"},
			Enabled:     &trueVal,
		},
		&testLogicalSwitchPort{
			UUID:        aUUID1,
			Name:        "lsp1",
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
			Name:        "lsp3",
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

	test := []struct {
		name      string
		predicate interface{}
		condition [][]ovsdb.Condition
		matches   map[model.Model]bool
		err       bool
	}{
		{
			name: "simple value comparison",
			predicate: func(lsp *testLogicalSwitchPort) bool {
				return lsp.UUID == aUUID0
			},
			condition: [][]ovsdb.Condition{
				{
					{
						Column:   "_uuid",
						Function: ovsdb.ConditionEqual,
						Value:    ovsdb.UUID{GoUUID: aUUID0},
					}}},
			matches: map[model.Model]bool{
				&testLogicalSwitchPort{UUID: aUUID0}:              true,
				&testLogicalSwitchPort{UUID: aUUID1}:              false,
				&testLogicalSwitchPort{UUID: aUUID0, Name: "foo"}: true,
			},
		},
		{
			name: "by random field",
			predicate: func(lsp *testLogicalSwitchPort) bool {
				return lsp.Enabled != nil && *lsp.Enabled == false
			},
			condition: [][]ovsdb.Condition{
				{
					{
						Column:   "_uuid",
						Function: ovsdb.ConditionEqual,
						Value:    ovsdb.UUID{GoUUID: aUUID1},
					}},
				{
					{
						Column:   "_uuid",
						Function: ovsdb.ConditionEqual,
						Value:    ovsdb.UUID{GoUUID: aUUID2},
					}}},
			matches: map[model.Model]bool{
				&testLogicalSwitchPort{UUID: aUUID1, Enabled: &trueVal}:  false,
				&testLogicalSwitchPort{UUID: aUUID1, Enabled: &falseVal}: true,
			},
		},
	}
	for _, tt := range test {
		t.Run(fmt.Sprintf("Predicate Conditional: %s", tt.name), func(t *testing.T) {
			cond, err := newPredicateConditional("Logical_Switch_Port", tcache, tt.predicate)
			assert.Nil(t, err)
			for model, shouldMatch := range tt.matches {
				matches, err := cond.Matches(model)
				if tt.err {
					assert.NotNil(t, err)
				} else {
					assert.Nil(t, err)
					assert.Equalf(t, shouldMatch, matches, fmt.Sprintf("Match on model %#+v should be %v", model, shouldMatch))
				}
			}
			generated, err := cond.Generate()
			if tt.err {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.ElementsMatch(t, tt.condition, generated)
			}
		})
	}
}

func TestExplicitConditional(t *testing.T) {
	lspcacheList := []model.Model{
		&testLogicalSwitchPort{
			UUID:        aUUID0,
			Name:        "lsp0",
			ExternalIds: map[string]string{"foo": "bar"},
			Enabled:     &trueVal,
		},
		&testLogicalSwitchPort{
			UUID:        aUUID1,
			Name:        "lsp1",
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
			Name:        "lsp3",
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

	testObj := &testLogicalSwitchPort{}

	test := []struct {
		name   string
		args   []model.Condition
		result [][]ovsdb.Condition
		all    bool
		err    bool
	}{
		{
			name: "inequality comparison",
			args: []model.Condition{
				{
					Field:    &testObj.Name,
					Function: ovsdb.ConditionNotEqual,
					Value:    "lsp0",
				},
			},
			result: [][]ovsdb.Condition{
				{
					{
						Column:   "name",
						Function: ovsdb.ConditionNotEqual,
						Value:    "lsp0",
					}}},
		},
		{
			name: "inequality comparison all",
			args: []model.Condition{
				{
					Field:    &testObj.Name,
					Function: ovsdb.ConditionNotEqual,
					Value:    "lsp0",
				},
			},
			result: [][]ovsdb.Condition{
				{
					{
						Column:   "name",
						Function: ovsdb.ConditionNotEqual,
						Value:    "lsp0",
					}}},
			all: true,
		},
		{
			name: "map comparison",
			args: []model.Condition{
				{
					Field:    &testObj.ExternalIds,
					Function: ovsdb.ConditionIncludes,
					Value:    map[string]string{"foo": "baz"},
				},
			},
			result: [][]ovsdb.Condition{
				{
					{
						Column:   "external_ids",
						Function: ovsdb.ConditionIncludes,
						Value:    testOvsMap(t, map[string]string{"foo": "baz"}),
					}}},
		},
		{
			name: "set comparison",
			args: []model.Condition{
				{
					Field:    &testObj.Enabled,
					Function: ovsdb.ConditionEqual,
					Value:    &trueVal,
				},
			},
			result: [][]ovsdb.Condition{
				{
					{
						Column:   "enabled",
						Function: ovsdb.ConditionEqual,
						Value:    testOvsSet(t, &trueVal),
					}}},
		},
		{
			name: "multiple conditions",
			args: []model.Condition{
				{
					Field:    &testObj.Enabled,
					Function: ovsdb.ConditionEqual,
					Value:    &trueVal,
				},
				{
					Field:    &testObj.Name,
					Function: ovsdb.ConditionNotEqual,
					Value:    "foo",
				},
			},
			result: [][]ovsdb.Condition{
				{
					{
						Column:   "enabled",
						Function: ovsdb.ConditionEqual,
						Value:    testOvsSet(t, &trueVal),
					}},
				{
					{
						Column:   "name",
						Function: ovsdb.ConditionNotEqual,
						Value:    "foo",
					}}},
		},
		{
			name: "multiple conditions all",
			args: []model.Condition{
				{
					Field:    &testObj.Enabled,
					Function: ovsdb.ConditionEqual,
					Value:    &trueVal,
				},
				{
					Field:    &testObj.Name,
					Function: ovsdb.ConditionNotEqual,
					Value:    "foo",
				},
			},
			result: [][]ovsdb.Condition{{
				{
					Column:   "enabled",
					Function: ovsdb.ConditionEqual,
					Value:    testOvsSet(t, &trueVal),
				},
				{
					Column:   "name",
					Function: ovsdb.ConditionNotEqual,
					Value:    "foo",
				}}},
			all: true,
		},
	}
	for _, tt := range test {
		t.Run(fmt.Sprintf("Explicit Conditional: %s", tt.name), func(t *testing.T) {
			cond, err := newExplicitConditional(tcache.Mapper(), "Logical_Switch_Port", tt.all, testObj, tt.args...)
			assert.Nil(t, err)
			_, err = cond.Matches(testObj)
			assert.NotNilf(t, err, "Explicit conditions should fail to match on cache")
			generated, err := cond.Generate()
			if tt.err {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.ElementsMatch(t, tt.result, generated)
			}
		})
	}
}
