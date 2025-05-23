package stmt

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"io"
	"reflect"
	"unsafe"

	"github.com/taosdata/driver-go/v3/common"
	"github.com/taosdata/driver-go/v3/common/parser"
	"github.com/taosdata/driver-go/v3/common/pointer"
	"github.com/taosdata/driver-go/v3/ws/client"
)

type Rows struct {
	buf              *bytes.Buffer
	blockPtr         unsafe.Pointer
	blockOffset      int
	blockSize        int
	resultID         uint64
	block            []byte
	conn             *WSConn
	client           *client.Client
	fieldsCount      int
	fieldsNames      []string
	fieldsTypes      []uint8
	fieldsLengths    []int64
	fieldsPrecisions []int64
	fieldsScales     []int64
	precision        int
}

func (rs *Rows) ColumnTypePrecisionScale(index int) (precision, scale int64, ok bool) {
	if rs.fieldsTypes[index] == common.TSDB_DATA_TYPE_DECIMAL || rs.fieldsTypes[index] == common.TSDB_DATA_TYPE_DECIMAL64 {
		return rs.fieldsPrecisions[index], rs.fieldsScales[index], true
	}
	return 0, 0, false
}

func (rs *Rows) Columns() []string {
	return rs.fieldsNames
}

func (rs *Rows) ColumnTypeDatabaseTypeName(i int) string {
	return common.GetTypeName(int(rs.fieldsTypes[i]))
}

func (rs *Rows) ColumnTypeLength(i int) (length int64, ok bool) {
	return rs.fieldsLengths[i], ok
}

func (rs *Rows) ColumnTypeScanType(i int) reflect.Type {
	t, exist := common.ColumnTypeMap[int(rs.fieldsTypes[i])]
	if !exist {
		return common.UnknownType
	}
	return t
}

func (rs *Rows) Close() error {
	rs.blockPtr = nil
	rs.block = nil
	return rs.freeResult()
}

func (rs *Rows) Next(dest []driver.Value) error {
	if rs.blockPtr == nil || rs.blockOffset >= rs.blockSize {
		err := rs.taosFetchBlock()
		if err != nil {
			return err
		}
	}
	if rs.blockSize == 0 {
		rs.blockPtr = nil
		rs.block = nil
		return io.EOF
	}
	err := parser.ReadRow(dest, rs.blockPtr, rs.blockSize, rs.blockOffset, rs.fieldsTypes, rs.precision, rs.fieldsScales)
	if err != nil {
		return err
	}
	rs.blockOffset += 1
	return nil
}

func (rs *Rows) taosFetchBlock() error {
	reqID := rs.conn.generateReqID()
	req := &WSFetchReq{
		ReqID: reqID,
		ID:    rs.resultID,
	}
	args, err := json.Marshal(req)
	if err != nil {
		return err
	}
	action := &client.WSAction{
		Action: WSFetch,
		Args:   args,
	}
	rs.buf.Reset()
	envelope := client.GlobalEnvelopePool.Get()
	defer client.GlobalEnvelopePool.Put(envelope)
	err = client.JsonI.NewEncoder(envelope.Msg).Encode(action)
	if err != nil {
		return err
	}
	respBytes, err := rs.conn.sendText(reqID, envelope)
	if err != nil {
		return err
	}
	var resp WSFetchResp
	err = client.JsonI.Unmarshal(respBytes, &resp)
	err = client.HandleResponseError(err, resp.Code, resp.Message)
	if err != nil {
		return err
	}
	if resp.Completed {
		rs.blockSize = 0
		return nil
	}
	rs.blockSize = resp.Rows
	return rs.fetchBlock()
}

func (rs *Rows) fetchBlock() error {
	req := &WSFetchBlockReq{
		ReqID: rs.resultID,
		ID:    rs.resultID,
	}
	args, err := client.JsonI.Marshal(req)
	if err != nil {
		return err
	}
	action := &client.WSAction{
		Action: WSFetchBlock,
		Args:   args,
	}
	rs.buf.Reset()
	envelope := client.GlobalEnvelopePool.Get()
	defer client.GlobalEnvelopePool.Put(envelope)
	err = client.JsonI.NewEncoder(envelope.Msg).Encode(action)
	if err != nil {
		return err
	}
	respBytes, err := rs.conn.sendText(rs.resultID, envelope)
	if err != nil {
		return err
	}
	rs.block = respBytes
	rs.blockPtr = pointer.AddUintptr(unsafe.Pointer(&rs.block[0]), 16)
	rs.blockOffset = 0
	return nil
}

func (rs *Rows) freeResult() error {
	reqID := rs.conn.generateReqID()
	req := &WSFreeResultRequest{
		ReqID: reqID,
		ID:    rs.resultID,
	}
	args, err := client.JsonI.Marshal(req)
	if err != nil {
		return err
	}
	action := &client.WSAction{
		Action: WSFreeResult,
		Args:   args,
	}
	rs.buf.Reset()
	envelope := client.GlobalEnvelopePool.Get()
	defer client.GlobalEnvelopePool.Put(envelope)
	err = client.JsonI.NewEncoder(envelope.Msg).Encode(action)
	if err != nil {
		return err
	}
	rs.conn.sendTextWithoutResp(envelope)
	return nil
}
