package mapper

import (
	"fmt"
	"reflect"

	"github.com/ovn-org/libovsdb/ovsdb"
)

// Info is a struct that handles the type map of an object
// The object must have exported tagged fields with the 'ovs'
type Info struct {
	// FieldName indexed by column
	fields map[string]string
	obj    interface{}
	table  *ovsdb.TableSchema
}

// FieldByColumn returns the field value that corresponds to a column
func (i *Info) FieldByColumn(column string) (interface{}, error) {
	fieldName, ok := i.fields[column]
	if !ok {
		return nil, fmt.Errorf("column %s not found in orm info", column)
	}
	return reflect.ValueOf(i.obj).Elem().FieldByName(fieldName).Interface(), nil
}

// FieldByColumn returns the field value that corresponds to a column
func (i *Info) hasColumn(column string) bool {
	_, ok := i.fields[column]
	return ok
}

// SetField sets the field in the column to the specified value
func (i *Info) SetField(column string, value interface{}) error {
	fieldName, ok := i.fields[column]
	if !ok {
		return fmt.Errorf("column %s not found in orm info", column)
	}
	fieldValue := reflect.ValueOf(i.obj).Elem().FieldByName(fieldName)
	v := reflect.ValueOf(value)
	if !fieldValue.Type().AssignableTo(reflect.TypeOf(value)) {
		if fieldValue.Kind() == reflect.Ptr {
			if v.Kind() == reflect.Ptr && fieldValue.Type().AssignableTo(reflect.TypeOf(v.Elem())) {
				v = v.Elem()
			} else {
				schema := i.table.Column(column)
				native, err := ovsdb.OvsToNative(schema, value)
				if err != nil {
					return err
				}
				v = reflect.ValueOf(native)
			}
		}
		if v.Type().ConvertibleTo(fieldValue.Type()) {
			// handle enum
			v = v.Convert(fieldValue.Type())
		} else if fieldValue.Kind() == reflect.Slice {
			// handle set of enums
			if !v.Type().Elem().ConvertibleTo(fieldValue.Type().Elem()) {
				return fmt.Errorf("column %s: element %v (%s) is not convertible to field %s element (%s)",
					column, value, reflect.TypeOf(value), fieldName, fieldValue.Type())
			}
			nv := reflect.Zero(fieldValue.Type())
			for i := 0; i < v.Len(); i++ {
				nv = reflect.Append(nv, v.Index(i).Convert(fieldValue.Type().Elem()))
			}
			v = nv
		} else {
			return fmt.Errorf("column %s: native value %v (%s) is not assignable or convertible to field %s (%s)",
				column, value, reflect.TypeOf(value), fieldName, fieldValue.Type())
		}
	}
	fieldValue.Set(v)
	return nil
}

// ColumnByPtr returns the column name that corresponds to the field by the field's pointer
func (i *Info) ColumnByPtr(fieldPtr interface{}) (string, error) {
	fieldPtrVal := reflect.ValueOf(fieldPtr)
	if fieldPtrVal.Kind() != reflect.Ptr {
		return "", ovsdb.NewErrWrongType("ColumnByPointer", "pointer to a field in the struct", fieldPtr)
	}
	offset := fieldPtrVal.Pointer() - reflect.ValueOf(i.obj).Pointer()
	objType := reflect.TypeOf(i.obj).Elem()
	for j := 0; j < objType.NumField(); j++ {
		if objType.Field(j).Offset == offset {
			column := objType.Field(j).Tag.Get("ovsdb")
			if _, ok := i.fields[column]; !ok {
				return "", fmt.Errorf("field does not have orm column information")
			}
			return column, nil
		}
	}
	return "", fmt.Errorf("field pointer does not correspond to orm struct")
}

// getValidIndexes inspects the object and returns the a list of indexes (set of columns) for witch
// the object has non-default values
func (i *Info) getValidIndexes() ([][]string, error) {
	var validIndexes [][]string
	var possibleIndexes [][]string

	possibleIndexes = append(possibleIndexes, []string{"_uuid"})
	possibleIndexes = append(possibleIndexes, i.table.Indexes...)

	// Iterate through indexes and validate them
OUTER:
	for _, idx := range possibleIndexes {
		for _, col := range idx {
			if !i.hasColumn(col) {
				continue OUTER
			}
			columnSchema := i.table.Column(col)
			if columnSchema == nil {
				continue OUTER
			}
			field, err := i.FieldByColumn(col)
			if err != nil {
				return nil, err
			}
			if !reflect.ValueOf(field).IsValid() || ovsdb.IsDefaultValue(columnSchema, field) {
				continue OUTER
			}
		}
		validIndexes = append(validIndexes, idx)
	}
	return validIndexes, nil
}

// NewInfo creates a MapperInfo structure around an object based on a given table schema
func NewInfo(table *ovsdb.TableSchema, obj interface{}) (*Info, error) {
	objPtrVal := reflect.ValueOf(obj)
	if objPtrVal.Type().Kind() != reflect.Ptr {
		return nil, ovsdb.NewErrWrongType("NewMapperInfo", "pointer to a struct", obj)
	}
	objVal := reflect.Indirect(objPtrVal)
	if objVal.Kind() != reflect.Struct {
		return nil, ovsdb.NewErrWrongType("NewMapperInfo", "pointer to a struct", obj)
	}
	objType := objVal.Type()

	fields := make(map[string]string, objType.NumField())
	for i := 0; i < objType.NumField(); i++ {
		field := objType.Field(i)
		colName := field.Tag.Get("ovsdb")
		if colName == "" {
			// Untagged fields are ignored
			continue
		}
		column := table.Column(colName)
		if column == nil {
			return nil, &ErrMapper{
				objType:   objType.String(),
				field:     field.Name,
				fieldType: field.Type.String(),
				fieldTag:  colName,
				reason:    "Column does not exist in schema",
			}
		}

		// Perform schema-based type checking
		expType := ovsdb.NativeType(column)
		// check for slice of enums
		if expType.Kind() == reflect.Slice && expType.Elem().Kind() == reflect.String {
			// it's a slice of enums
		} else if expType.Kind() == reflect.String && field.Type.Kind() == reflect.String {
			// it's an enum
		} else if expType.Kind() == reflect.Ptr && expType.Elem().Kind() == reflect.String && field.Type.Kind() == reflect.Ptr && field.Type.Elem().Kind() == reflect.String {
			// it's a pointer to an enum
		} else if expType != field.Type {
			return nil, &ErrMapper{
				objType:   objType.String(),
				field:     field.Name,
				fieldType: field.Type.String(),
				fieldTag:  colName,
				reason:    fmt.Sprintf("Wrong type, column expects %s", expType),
			}
		}
		fields[colName] = field.Name
	}

	return &Info{
		fields: fields,
		obj:    obj,
		table:  table,
	}, nil
}
