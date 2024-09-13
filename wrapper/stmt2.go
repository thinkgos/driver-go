package wrapper

/*
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <taos.h>

extern void Stmt2ExecCallback(void *param,TAOS_RES *,int code);
//TAOS_STMT2 *taos_stmt2_init(TAOS *taos, TAOS_STMT2_OPTION *option);
TAOS_STMT2 * taos_stmt2_init_wrapper(TAOS *taos, int64_t reqid, bool singleStbInsert,bool singleTableBindOnce, void *param){
	TAOS_STMT2_OPTION option = {reqid, singleStbInsert, singleTableBindOnce, Stmt2ExecCallback , param};
	return taos_stmt2_init(taos,&option);
};
*/
import "C"
import (
	"database/sql/driver"
	"fmt"
	"time"
	"unsafe"

	"github.com/taosdata/driver-go/v3/common"
	"github.com/taosdata/driver-go/v3/common/stmt"
	taosError "github.com/taosdata/driver-go/v3/errors"
	"github.com/taosdata/driver-go/v3/wrapper/cgo"
)

// TaosStmt2Init TAOS_STMT2 *taos_stmt2_init(TAOS *taos, TAOS_STMT2_OPTION *option);
func TaosStmt2Init(taosConnect unsafe.Pointer, reqID int64, singleStbInsert bool, singleTableBindOnce bool, caller cgo.Handle) unsafe.Pointer {
	return C.taos_stmt2_init_wrapper(taosConnect, C.int64_t(reqID), C.bool(singleStbInsert), C.bool(singleTableBindOnce), caller.Pointer())
}

// TaosStmt2Prepare int taos_stmt2_prepare(TAOS_STMT2 *stmt, const char *sql, unsigned long length);
func TaosStmt2Prepare(stmt unsafe.Pointer, sql string) int {
	cSql := C.CString(sql)
	cLen := C.ulong(len(sql))
	defer C.free(unsafe.Pointer(cSql))
	return int(C.taos_stmt2_prepare(stmt, cSql, cLen))
}

// TaosStmt2BindParam int         taos_stmt2_bind_param(TAOS_STMT2 *stmt, TAOS_STMT2_BINDV *bindv, int32_t col_idx);
func TaosStmt2BindParam(stmt unsafe.Pointer, isInsert bool, params []*stmt.TaosStmt2BindData, colTypes, tagTypes []*stmt.StmtField, colIdx int32) error {
	count := len(params)
	if count == 0 {
		return taosError.NewError(0xffff, "params is empty")
	}
	cBindv := C.TAOS_STMT2_BINDV{}
	cBindv.count = C.int(count)
	tbNames := C.malloc(C.size_t(count) * C.size_t(PointerSize))
	needFreePointer := []unsafe.Pointer{unsafe.Pointer(tbNames)}
	tbNamesSlice := (*[1 << 30]*C.char)(unsafe.Pointer(tbNames))[:count:count]
	tagList := C.malloc(C.size_t(count) * C.size_t(PointerSize))
	needFreePointer = append(needFreePointer, unsafe.Pointer(tagList))
	tagArray := (*[1 << 30]*C.TAOS_STMT2_BIND)(unsafe.Pointer(tagList))[:count:count]
	colList := C.malloc(C.size_t(count) * C.size_t(PointerSize))
	needFreePointer = append(needFreePointer, unsafe.Pointer(colList))
	colArray := (*[1 << 30]*C.TAOS_STMT2_BIND)(unsafe.Pointer(colList))[:count:count]
	defer func() {
		for _, pointer := range needFreePointer {
			if pointer != nil {
				C.free(pointer)
			}
		}
	}()

	for i, param := range params {
		//parse table name
		if param.TableName != "" {
			if !isInsert {
				return taosError.NewError(0xffff, "table name is not allowed in query statement")
			}
			tbName := C.CString(param.TableName)
			tbNamesSlice[i] = tbName
			needFreePointer = append(needFreePointer, unsafe.Pointer(tbName))
		} else {
			tbNamesSlice[i] = nil
		}
		//parse tags
		if len(param.Tags) > 0 {
			if !isInsert {
				return taosError.NewError(0xffff, "tag is not allowed in query statement")
			}
			//transpose
			columnFormatTags := make([][]driver.Value, len(param.Tags))
			for j := 0; j < len(param.Tags); j++ {
				columnFormatTags[j] = []driver.Value{param.Tags[j]}
			}
			tags, err := generateTaosStmt2BindsInsert(columnFormatTags, tagTypes, &needFreePointer)
			if err != nil {
				return taosError.NewError(0xffff, fmt.Sprintf("generate tags Bindv struct error: %s", err.Error()))
			}
			tagArray[i] = &(tags[0])
		} else {
			tagArray[i] = nil
		}
		// parse cols
		if len(param.Cols) > 0 {
			var err error
			var cols []C.TAOS_STMT2_BIND
			if isInsert {
				cols, err = generateTaosStmt2BindsInsert(param.Cols, colTypes, &needFreePointer)
			} else {
				cols, err = generateTaosStmt2BindsQuery(param.Cols, &needFreePointer)
			}
			if err != nil {
				return taosError.NewError(0xffff, fmt.Sprintf("generate cols Bindv struct error: %s", err.Error()))
			}
			colArray[i] = &(cols[0])
		} else {
			colArray[i] = nil
		}
	}
	cBindv.bind_cols = (**C.TAOS_STMT2_BIND)(unsafe.Pointer(colList))
	cBindv.tags = (**C.TAOS_STMT2_BIND)(unsafe.Pointer(tagList))
	cBindv.tbnames = (**C.char)(tbNames)

	code := int(C.taos_stmt2_bind_param(stmt, &cBindv, C.int32_t(colIdx)))
	if code != 0 {
		errStr := TaosStmt2Error(stmt)
		return taosError.NewError(code, errStr)
	}
	return nil
}

func generateTaosStmt2BindsInsert(multiBind [][]driver.Value, fieldTypes []*stmt.StmtField, needFreePointer *[]unsafe.Pointer) ([]C.TAOS_STMT2_BIND, error) {
	if len(multiBind) != len(fieldTypes) {
		return nil, fmt.Errorf("data and type length not match, data length: %d, type length: %d", len(multiBind), len(fieldTypes))
	}
	binds := C.malloc(C.size_t(len(multiBind)) * C.sizeof_struct_TAOS_STMT2_BIND)
	*needFreePointer = append(*needFreePointer, unsafe.Pointer(binds))
	bindsSlice := (*[1 << 30]C.TAOS_STMT2_BIND)(unsafe.Pointer(binds))[:len(multiBind):len(multiBind)]
	for columnIndex, columnData := range multiBind {
		rowLen := len(multiBind[0])
		bindsSlice[columnIndex].num = C.int(rowLen)
		nullList := unsafe.Pointer(C.malloc(C.size_t(C.uint(rowLen))))
		*needFreePointer = append(*needFreePointer, nullList)
		lengthList := unsafe.Pointer(C.calloc(C.size_t(C.uint(rowLen)), C.size_t(C.uint(4))))
		*needFreePointer = append(*needFreePointer, lengthList)
		var p unsafe.Pointer
		*needFreePointer = append(*needFreePointer, p)
		columnType := fieldTypes[columnIndex].FieldType
		precision := int(fieldTypes[columnIndex].Precision)
		switch columnType {
		case common.TSDB_DATA_TYPE_BOOL:
			//1
			p = unsafe.Pointer(C.malloc(C.size_t(C.uint(rowLen))))
			bindsSlice[columnIndex].buffer_type = C.TSDB_DATA_TYPE_BOOL
			for i, rowData := range columnData {
				currentNull := unsafe.Pointer(uintptr(nullList) + uintptr(i))
				if rowData == nil {
					*(*C.char)(currentNull) = C.char(1)
				} else {
					*(*C.char)(currentNull) = C.char(0)
					value, ok := rowData.(bool)
					if !ok {
						return nil, fmt.Errorf("data type error, expect bool, but got %T, value: %v", rowData, value)
					}
					current := unsafe.Pointer(uintptr(p) + uintptr(i))
					if value {
						*(*C.int8_t)(current) = C.int8_t(1)
					} else {
						*(*C.int8_t)(current) = C.int8_t(0)
					}

					l := unsafe.Pointer(uintptr(lengthList) + uintptr(4*i))
					*(*C.int32_t)(l) = C.int32_t(1)
				}
			}
		case common.TSDB_DATA_TYPE_TINYINT:
			//1
			p = unsafe.Pointer(C.malloc(C.size_t(C.uint(rowLen))))
			bindsSlice[columnIndex].buffer_type = C.TSDB_DATA_TYPE_TINYINT
			for i, rowData := range columnData {
				currentNull := unsafe.Pointer(uintptr(nullList) + uintptr(i))
				if rowData == nil {
					*(*C.char)(currentNull) = C.char(1)
				} else {
					*(*C.char)(currentNull) = C.char(0)
					value, ok := rowData.(int8)
					if !ok {
						return nil, fmt.Errorf("data type error, expect int8, but got %T, value: %v", rowData, value)
					}
					current := unsafe.Pointer(uintptr(p) + uintptr(i))
					*(*C.int8_t)(current) = C.int8_t(value)

					l := unsafe.Pointer(uintptr(lengthList) + uintptr(4*i))
					*(*C.int32_t)(l) = C.int32_t(1)
				}
			}
		case common.TSDB_DATA_TYPE_SMALLINT:
			//2
			p = unsafe.Pointer(C.malloc(C.size_t(C.uint(2 * rowLen))))
			bindsSlice[columnIndex].buffer_type = C.TSDB_DATA_TYPE_SMALLINT
			for i, rowData := range columnData {
				currentNull := unsafe.Pointer(uintptr(nullList) + uintptr(i))
				if rowData == nil {
					*(*C.char)(currentNull) = C.char(1)
				} else {
					*(*C.char)(currentNull) = C.char(0)
					value, ok := rowData.(int16)
					if !ok {
						return nil, fmt.Errorf("data type error, expect int16, but got %T, value: %v", rowData, value)
					}
					current := unsafe.Pointer(uintptr(p) + uintptr(2*i))
					*(*C.int16_t)(current) = C.int16_t(value)

					l := unsafe.Pointer(uintptr(lengthList) + uintptr(4*i))
					*(*C.int32_t)(l) = C.int32_t(2)
				}
			}
		case common.TSDB_DATA_TYPE_INT:
			//4
			p = unsafe.Pointer(C.malloc(C.size_t(C.uint(4 * rowLen))))
			bindsSlice[columnIndex].buffer_type = C.TSDB_DATA_TYPE_INT
			for i, rowData := range columnData {
				currentNull := unsafe.Pointer(uintptr(nullList) + uintptr(i))
				if rowData == nil {
					*(*C.char)(currentNull) = C.char(1)
				} else {
					*(*C.char)(currentNull) = C.char(0)
					value, ok := rowData.(int32)
					if !ok {
						return nil, fmt.Errorf("data type error, expect int32, but got %T, value: %v", rowData, value)
					}
					current := unsafe.Pointer(uintptr(p) + uintptr(4*i))
					*(*C.int32_t)(current) = C.int32_t(value)

					l := unsafe.Pointer(uintptr(lengthList) + uintptr(4*i))
					*(*C.int32_t)(l) = C.int32_t(4)
				}
			}
		case common.TSDB_DATA_TYPE_BIGINT:
			//8
			p = unsafe.Pointer(C.malloc(C.size_t(C.uint(8 * rowLen))))
			bindsSlice[columnIndex].buffer_type = C.TSDB_DATA_TYPE_BIGINT
			for i, rowData := range columnData {
				currentNull := unsafe.Pointer(uintptr(nullList) + uintptr(i))
				if rowData == nil {
					*(*C.char)(currentNull) = C.char(1)
				} else {
					*(*C.char)(currentNull) = C.char(0)
					value, ok := rowData.(int64)
					if !ok {
						return nil, fmt.Errorf("data type error, expect int64, but got %T, value: %v", rowData, value)
					}
					current := unsafe.Pointer(uintptr(p) + uintptr(8*i))
					*(*C.int64_t)(current) = C.int64_t(value)

					l := unsafe.Pointer(uintptr(lengthList) + uintptr(4*i))
					*(*C.int32_t)(l) = C.int32_t(8)
				}
			}
		case common.TSDB_DATA_TYPE_UTINYINT:
			//1
			p = unsafe.Pointer(C.malloc(C.size_t(C.uint(rowLen))))
			bindsSlice[columnIndex].buffer_type = C.TSDB_DATA_TYPE_UTINYINT
			for i, rowData := range columnData {
				currentNull := unsafe.Pointer(uintptr(nullList) + uintptr(i))
				if rowData == nil {
					*(*C.char)(currentNull) = C.char(1)
				} else {
					*(*C.char)(currentNull) = C.char(0)
					value, ok := rowData.(uint8)
					if !ok {
						return nil, fmt.Errorf("data type error, expect uint8, but got %T, value: %v", rowData, value)
					}
					current := unsafe.Pointer(uintptr(p) + uintptr(i))
					*(*C.uint8_t)(current) = C.uint8_t(value)

					l := unsafe.Pointer(uintptr(lengthList) + uintptr(4*i))
					*(*C.int32_t)(l) = C.int32_t(1)
				}
			}
		case common.TSDB_DATA_TYPE_USMALLINT:
			//2
			p = unsafe.Pointer(C.malloc(C.size_t(C.uint(2 * rowLen))))
			bindsSlice[columnIndex].buffer_type = C.TSDB_DATA_TYPE_USMALLINT
			for i, rowData := range columnData {
				currentNull := unsafe.Pointer(uintptr(nullList) + uintptr(i))
				if rowData == nil {
					*(*C.char)(currentNull) = C.char(1)
				} else {
					*(*C.char)(currentNull) = C.char(0)
					value, ok := rowData.(uint16)
					if !ok {
						return nil, fmt.Errorf("data type error, expect uint16, but got %T, value: %v", rowData, value)
					}
					current := unsafe.Pointer(uintptr(p) + uintptr(2*i))
					*(*C.uint16_t)(current) = C.uint16_t(value)

					l := unsafe.Pointer(uintptr(lengthList) + uintptr(4*i))
					*(*C.int32_t)(l) = C.int32_t(2)
				}
			}
		case common.TSDB_DATA_TYPE_UINT:
			//4
			p = unsafe.Pointer(C.malloc(C.size_t(C.uint(4 * rowLen))))
			bindsSlice[columnIndex].buffer_type = C.TSDB_DATA_TYPE_UINT
			for i, rowData := range columnData {
				currentNull := unsafe.Pointer(uintptr(nullList) + uintptr(i))
				if rowData == nil {
					*(*C.char)(currentNull) = C.char(1)
				} else {
					*(*C.char)(currentNull) = C.char(0)
					value, ok := rowData.(uint32)
					if !ok {
						return nil, fmt.Errorf("data type error, expect uint32, but got %T, value: %v", rowData, value)
					}
					current := unsafe.Pointer(uintptr(p) + uintptr(4*i))
					*(*C.uint32_t)(current) = C.uint32_t(value)

					l := unsafe.Pointer(uintptr(lengthList) + uintptr(4*i))
					*(*C.int32_t)(l) = C.int32_t(4)
				}
			}
		case common.TSDB_DATA_TYPE_UBIGINT:
			//8
			p = unsafe.Pointer(C.malloc(C.size_t(C.uint(8 * rowLen))))
			bindsSlice[columnIndex].buffer_type = C.TSDB_DATA_TYPE_UBIGINT
			for i, rowData := range columnData {
				currentNull := unsafe.Pointer(uintptr(nullList) + uintptr(i))
				if rowData == nil {
					*(*C.char)(currentNull) = C.char(1)
				} else {
					*(*C.char)(currentNull) = C.char(0)
					value, ok := rowData.(uint64)
					if !ok {
						return nil, fmt.Errorf("data type error, expect uint64, but got %T, value: %v", rowData, value)
					}
					current := unsafe.Pointer(uintptr(p) + uintptr(8*i))
					*(*C.uint64_t)(current) = C.uint64_t(value)

					l := unsafe.Pointer(uintptr(lengthList) + uintptr(4*i))
					*(*C.int32_t)(l) = C.int32_t(8)
				}
			}
		case common.TSDB_DATA_TYPE_FLOAT:
			//4
			p = unsafe.Pointer(C.malloc(C.size_t(C.uint(4 * rowLen))))
			bindsSlice[columnIndex].buffer_type = C.TSDB_DATA_TYPE_FLOAT
			for i, rowData := range columnData {
				currentNull := unsafe.Pointer(uintptr(nullList) + uintptr(i))
				if rowData == nil {
					*(*C.char)(currentNull) = C.char(1)
				} else {
					*(*C.char)(currentNull) = C.char(0)
					value, ok := rowData.(float32)
					if !ok {
						return nil, fmt.Errorf("data type error, expect float32, but got %T, value: %v", rowData, value)
					}
					current := unsafe.Pointer(uintptr(p) + uintptr(4*i))
					*(*C.float)(current) = C.float(value)

					l := unsafe.Pointer(uintptr(lengthList) + uintptr(4*i))
					*(*C.int32_t)(l) = C.int32_t(4)
				}
			}
		case common.TSDB_DATA_TYPE_DOUBLE:
			//8
			p = unsafe.Pointer(C.malloc(C.size_t(C.uint(8 * rowLen))))
			bindsSlice[columnIndex].buffer_type = C.TSDB_DATA_TYPE_DOUBLE
			for i, rowData := range columnData {
				currentNull := unsafe.Pointer(uintptr(nullList) + uintptr(i))
				if rowData == nil {
					*(*C.char)(currentNull) = C.char(1)
					l := unsafe.Pointer(uintptr(lengthList) + uintptr(4*i))
					*(*C.int32_t)(l) = C.int32_t(0)
				} else {
					*(*C.char)(currentNull) = C.char(0)
					value, ok := rowData.(float64)
					if !ok {
						return nil, fmt.Errorf("data type error, expect float64, but got %T, value: %v", rowData, value)
					}
					current := unsafe.Pointer(uintptr(p) + uintptr(8*i))
					*(*C.double)(current) = C.double(value)

					l := unsafe.Pointer(uintptr(lengthList) + uintptr(4*i))
					*(*C.int32_t)(l) = C.int32_t(8)
				}
			}
		case common.TSDB_DATA_TYPE_BINARY, common.TSDB_DATA_TYPE_VARBINARY, common.TSDB_DATA_TYPE_JSON, common.TSDB_DATA_TYPE_GEOMETRY, common.TSDB_DATA_TYPE_NCHAR:
			bindsSlice[columnIndex].buffer_type = C.int(columnType)
			colOffset := make([]int, rowLen)
			totalLen := 0
			for i, rowData := range columnData {
				currentNull := unsafe.Pointer(uintptr(nullList) + uintptr(i))
				if rowData == nil {
					*(*C.char)(currentNull) = C.char(1)
					l := unsafe.Pointer(uintptr(lengthList) + uintptr(4*i))

					*(*C.int32_t)(l) = C.int32_t(0)
				} else {
					colOffset[i] = totalLen
					switch value := rowData.(type) {
					case string:
						totalLen += len(value)
						l := unsafe.Pointer(uintptr(lengthList) + uintptr(4*i))
						*(*C.int32_t)(l) = C.int32_t(len(value))
					case []byte:
						totalLen += len(value)
						l := unsafe.Pointer(uintptr(lengthList) + uintptr(4*i))
						*(*C.int32_t)(l) = C.int32_t(len(value))
					default:
						return nil, fmt.Errorf("data type error, expect string or []byte, but got %T, value: %v", rowData, value)
					}
					*(*C.char)(currentNull) = C.char(0)
				}
			}
			p = unsafe.Pointer(C.malloc(C.size_t(C.uint(totalLen))))
			for i, rowData := range columnData {
				if rowData != nil {
					switch value := rowData.(type) {
					case string:
						x := *(*[]byte)(unsafe.Pointer(&value))
						C.memcpy(unsafe.Pointer(uintptr(p)+uintptr(colOffset[i])), unsafe.Pointer(&x[0]), C.size_t(len(value)))
					case []byte:
						C.memcpy(unsafe.Pointer(uintptr(p)+uintptr(colOffset[i])), unsafe.Pointer(&value[0]), C.size_t(len(value)))
					default:
						return nil, fmt.Errorf("data type error, expect string or []byte, but got %T, value: %v", rowData, value)
					}
				}
			}
		case common.TSDB_DATA_TYPE_TIMESTAMP:
			//8
			p = unsafe.Pointer(C.malloc(C.size_t(C.uint(8 * rowLen))))
			bindsSlice[columnIndex].buffer_type = C.TSDB_DATA_TYPE_TIMESTAMP
			for i, rowData := range columnData {
				currentNull := unsafe.Pointer(uintptr(nullList) + uintptr(i))
				if rowData == nil {
					*(*C.char)(currentNull) = C.char(1)
					l := unsafe.Pointer(uintptr(lengthList) + uintptr(4*i))
					*(*C.int32_t)(l) = C.int32_t(0)
				} else {
					*(*C.char)(currentNull) = C.char(0)
					value, ok := rowData.(time.Time)
					if !ok {
						return nil, fmt.Errorf("data type error, expect time.Time, but got %T, value: %v", rowData, value)
					}
					ts := common.TimeToTimestamp(value, precision)
					current := unsafe.Pointer(uintptr(p) + uintptr(8*i))
					*(*C.int64_t)(current) = C.int64_t(ts)

					l := unsafe.Pointer(uintptr(lengthList) + uintptr(4*i))
					*(*C.int32_t)(l) = C.int32_t(8)
				}
			}
		}
		bindsSlice[columnIndex].buffer = p
		bindsSlice[columnIndex].length = (*C.int32_t)(lengthList)
		bindsSlice[columnIndex].is_null = (*C.char)(nullList)
	}

	return bindsSlice, nil

}

func generateTaosStmt2BindsQuery(multiBind [][]driver.Value, needFreePointer *[]unsafe.Pointer) ([]C.TAOS_STMT2_BIND, error) {
	binds := C.malloc(C.size_t(len(multiBind)) * C.sizeof_struct_TAOS_STMT2_BIND)
	*needFreePointer = append(*needFreePointer, unsafe.Pointer(binds))
	bindsSlice := (*[1 << 30]C.TAOS_STMT2_BIND)(unsafe.Pointer(binds))[:len(multiBind):len(multiBind)]
	for columnIndex, columnData := range multiBind {
		if len(columnData) != 1 {
			return nil, fmt.Errorf("bind query data length must be 1, but column %d got %d", columnIndex, len(columnData))
		}
		data := columnData[0]
		bindsSlice[columnIndex].num = C.int(1)
		nullList := unsafe.Pointer(C.malloc(C.size_t(C.uint(1))))
		*needFreePointer = append(*needFreePointer, nullList)
		var lengthList unsafe.Pointer
		*needFreePointer = append(*needFreePointer, lengthList)
		var p unsafe.Pointer
		*needFreePointer = append(*needFreePointer, p)

		if data == nil {
			return nil, fmt.Errorf("bind query data can not be nil")
		}
		*(*C.char)(nullList) = C.char(0)

		switch rowData := data.(type) {
		case bool:
			p = unsafe.Pointer(C.malloc(C.size_t(C.uint(1))))
			bindsSlice[columnIndex].buffer_type = C.TSDB_DATA_TYPE_BOOL
			if rowData {
				*(*C.int8_t)(p) = C.int8_t(1)
			} else {
				*(*C.int8_t)(p) = C.int8_t(0)
			}

		case int8:
			p = unsafe.Pointer(C.malloc(C.size_t(C.uint(1))))
			bindsSlice[columnIndex].buffer_type = C.TSDB_DATA_TYPE_TINYINT
			*(*C.int8_t)(p) = C.int8_t(rowData)

		case int16:
			p = unsafe.Pointer(C.malloc(C.size_t(C.uint(2))))
			bindsSlice[columnIndex].buffer_type = C.TSDB_DATA_TYPE_SMALLINT
			*(*C.int16_t)(p) = C.int16_t(rowData)

		case int32:
			p = unsafe.Pointer(C.malloc(C.size_t(C.uint(4))))
			bindsSlice[columnIndex].buffer_type = C.TSDB_DATA_TYPE_INT
			*(*C.int32_t)(p) = C.int32_t(rowData)

		case int64:
			p = unsafe.Pointer(C.malloc(C.size_t(C.uint(8))))
			bindsSlice[columnIndex].buffer_type = C.TSDB_DATA_TYPE_BIGINT
			*(*C.int64_t)(p) = C.int64_t(rowData)

		case int:
			p = unsafe.Pointer(C.malloc(C.size_t(C.uint(8))))
			bindsSlice[columnIndex].buffer_type = C.TSDB_DATA_TYPE_BIGINT
			*(*C.int64_t)(p) = C.int64_t(int64(rowData))

		case uint8:
			p = unsafe.Pointer(C.malloc(C.size_t(C.uint(1))))
			bindsSlice[columnIndex].buffer_type = C.TSDB_DATA_TYPE_UTINYINT
			*(*C.uint8_t)(p) = C.uint8_t(rowData)

		case uint16:
			p = unsafe.Pointer(C.malloc(C.size_t(C.uint(2))))
			bindsSlice[columnIndex].buffer_type = C.TSDB_DATA_TYPE_USMALLINT
			*(*C.uint16_t)(p) = C.uint16_t(rowData)

		case uint32:
			p = unsafe.Pointer(C.malloc(C.size_t(C.uint(4))))
			bindsSlice[columnIndex].buffer_type = C.TSDB_DATA_TYPE_UINT
			*(*C.uint32_t)(p) = C.uint32_t(rowData)

		case uint64:
			p = unsafe.Pointer(C.malloc(C.size_t(C.uint(8))))
			bindsSlice[columnIndex].buffer_type = C.TSDB_DATA_TYPE_UBIGINT
			*(*C.uint64_t)(p) = C.uint64_t(rowData)

		case uint:
			p = unsafe.Pointer(C.malloc(C.size_t(C.uint(8))))
			bindsSlice[columnIndex].buffer_type = C.TSDB_DATA_TYPE_UBIGINT
			*(*C.uint64_t)(p) = C.uint64_t(uint64(rowData))

		case float32:
			p = unsafe.Pointer(C.malloc(C.size_t(C.uint(4))))
			bindsSlice[columnIndex].buffer_type = C.TSDB_DATA_TYPE_FLOAT
			*(*C.float)(p) = C.float(rowData)

		case float64:
			p = unsafe.Pointer(C.malloc(C.size_t(C.uint(8))))
			bindsSlice[columnIndex].buffer_type = C.TSDB_DATA_TYPE_DOUBLE
			*(*C.double)(p) = C.double(rowData)

		case []byte:
			valueLength := len(rowData)
			p = unsafe.Pointer(C.malloc(C.size_t(C.uint(valueLength))))
			bindsSlice[columnIndex].buffer_type = C.TSDB_DATA_TYPE_BINARY
			C.memcpy(p, unsafe.Pointer(&rowData[0]), C.size_t(valueLength))
			lengthList = unsafe.Pointer(C.calloc(C.size_t(C.uint(1)), C.size_t(C.uint(4))))
			*(*C.int32_t)(lengthList) = C.int32_t(valueLength)

		case string:
			valueLength := len(rowData)
			p = unsafe.Pointer(C.malloc(C.size_t(C.uint(valueLength))))
			bindsSlice[columnIndex].buffer_type = C.TSDB_DATA_TYPE_BINARY
			x := *(*[]byte)(unsafe.Pointer(&rowData))
			C.memcpy(p, unsafe.Pointer(&x[0]), C.size_t(valueLength))
			lengthList = unsafe.Pointer(C.calloc(C.size_t(C.uint(1)), C.size_t(C.uint(4))))
			*(*C.int32_t)(lengthList) = C.int32_t(valueLength)
		case time.Time:
			buffer := make([]byte, 0, 35)
			value := rowData.AppendFormat(buffer, time.RFC3339Nano)
			valueLength := len(value)
			p = unsafe.Pointer(C.malloc(C.size_t(C.uint(valueLength))))
			bindsSlice[columnIndex].buffer_type = C.TSDB_DATA_TYPE_BINARY
			x := *(*[]byte)(unsafe.Pointer(&value))
			C.memcpy(p, unsafe.Pointer(&x[0]), C.size_t(valueLength))
			lengthList = unsafe.Pointer(C.calloc(C.size_t(C.uint(1)), C.size_t(C.uint(4))))
			*(*C.int32_t)(lengthList) = C.int32_t(valueLength)
		default:
			return nil, fmt.Errorf("data type error, expect bool, int8, int16, int32, int64, uint8, uint16, uint32, uint64, float32, float64, []byte, string, time.Time, but got %T, value: %v", data, data)
		}
		bindsSlice[columnIndex].buffer = p
		bindsSlice[columnIndex].length = (*C.int32_t)(lengthList)
		bindsSlice[columnIndex].is_null = (*C.char)(nullList)
	}
	return bindsSlice, nil
}

// TaosStmt2Exec int taos_stmt2_exec(TAOS_STMT2 *stmt, int *affected_rows);
func TaosStmt2Exec(stmt unsafe.Pointer) int {
	return int(C.taos_stmt2_exec(stmt, nil))
}

// TaosStmt2Close int taos_stmt2_close(TAOS_STMT2 *stmt);
func TaosStmt2Close(stmt unsafe.Pointer) int {
	return int(C.taos_stmt2_close(stmt))
}

// TaosStmt2IsInsert int taos_stmt2_is_insert(TAOS_STMT2 *stmt, int *insert);
func TaosStmt2IsInsert(stmt unsafe.Pointer) (is bool, errorCode int) {
	p := C.malloc(C.size_t(4))
	isInsert := (*C.int)(p)
	defer C.free(p)
	errorCode = int(C.taos_stmt2_is_insert(stmt, isInsert))
	return int(*isInsert) == 1, errorCode
}

// TaosStmt2GetFields int  taos_stmt2_get_fields(TAOS_STMT2 *stmt, TAOS_FIELD_T field_type, int *count, TAOS_FIELD_E **fields);
func TaosStmt2GetFields(stmt unsafe.Pointer, fieldType int) (code, count int, fields unsafe.Pointer) {
	code = int(C.taos_stmt2_get_fields(stmt, C.TAOS_FIELD_T(fieldType), (*C.int)(unsafe.Pointer(&count)), (**C.TAOS_FIELD_E)(unsafe.Pointer(&fields))))
	return
}

// TaosStmt2FreeFields void taos_stmt2_free_fields(TAOS_STMT2 *stmt, TAOS_FIELD_E *fields);
func TaosStmt2FreeFields(stmt unsafe.Pointer, fields unsafe.Pointer) {
	C.taos_stmt2_free_fields(stmt, (*C.TAOS_FIELD_E)(fields))
}

// TaosStmt2Error char     *taos_stmt2_error(TAOS_STMT2 *stmt)
func TaosStmt2Error(stmt unsafe.Pointer) string {
	return C.GoString(C.taos_stmt2_error(stmt))
}
