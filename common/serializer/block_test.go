package serializer

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/taosdata/driver-go/v3/common/param"
)

// @author: xftan
// @date: 2023/10/13 11:19
// @description: test block
func TestSerializeRawBlock(t *testing.T) {
	type args struct {
		params  []*param.Param
		colType *param.ColumnType
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name: "all type",
			args: args{
				params: []*param.Param{
					param.NewParam(1).AddTimestamp(time.Unix(0, 0), 0),
					param.NewParam(1).AddBool(true),
					param.NewParam(1).AddTinyint(127),
					param.NewParam(1).AddSmallint(32767),
					param.NewParam(1).AddInt(2147483647),
					param.NewParam(1).AddBigint(9223372036854775807),
					param.NewParam(1).AddUTinyint(255),
					param.NewParam(1).AddUSmallint(65535),
					param.NewParam(1).AddUInt(4294967295),
					param.NewParam(1).AddUBigint(18446744073709551615),
					param.NewParam(1).AddFloat(math.MaxFloat32),
					param.NewParam(1).AddDouble(math.MaxFloat64),
					param.NewParam(1).AddBinary([]byte("ABC")),
					param.NewParam(1).AddNchar("涛思数据"),
				},
				colType: param.NewColumnType(14).
					AddTimestamp().
					AddBool().
					AddTinyint().
					AddSmallint().
					AddInt().
					AddBigint().
					AddUTinyint().
					AddUSmallint().
					AddUInt().
					AddUBigint().
					AddFloat().
					AddDouble().
					AddBinary(0).
					AddNchar(0),
			},
			want: []byte{
				0x01, 0x00, 0x00, 0x00, //version
				0xf8, 0x00, 0x00, 0x00, //length
				0x01, 0x00, 0x00, 0x00, //rows
				0x0e, 0x00, 0x00, 0x00, //columns
				0x00, 0x00, 0x00, 0x00, //flagSegment
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, //groupID
				//types
				0x09, 0x08, 0x00, 0x00, 0x00, //1
				0x01, 0x01, 0x00, 0x00, 0x00, //2
				0x02, 0x01, 0x00, 0x00, 0x00, //3
				0x03, 0x02, 0x00, 0x00, 0x00, //4
				0x04, 0x04, 0x00, 0x00, 0x00, //5
				0x05, 0x08, 0x00, 0x00, 0x00, //6
				0x0b, 0x01, 0x00, 0x00, 0x00, //7
				0x0c, 0x02, 0x00, 0x00, 0x00, //8
				0x0d, 0x04, 0x00, 0x00, 0x00, //9
				0x0e, 0x08, 0x00, 0x00, 0x00, //10
				0x06, 0x04, 0x00, 0x00, 0x00, //11
				0x07, 0x08, 0x00, 0x00, 0x00, //12
				0x08, 0x00, 0x00, 0x00, 0x00, //13
				0x0a, 0x00, 0x00, 0x00, 0x00, //14
				//lengths
				0x08, 0x00, 0x00, 0x00,
				0x01, 0x00, 0x00, 0x00,
				0x01, 0x00, 0x00, 0x00,
				0x02, 0x00, 0x00, 0x00,
				0x04, 0x00, 0x00, 0x00,
				0x08, 0x00, 0x00, 0x00,
				0x01, 0x00, 0x00, 0x00,
				0x02, 0x00, 0x00, 0x00,
				0x04, 0x00, 0x00, 0x00,
				0x08, 0x00, 0x00, 0x00,
				0x04, 0x00, 0x00, 0x00,
				0x08, 0x00, 0x00, 0x00,
				0x05, 0x00, 0x00, 0x00,
				0x12, 0x00, 0x00, 0x00,
				0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, //ts
				0x00,
				0x01, //bool
				0x00,
				0x7f, //i8
				0x00,
				0xff, 0x7f, //i16
				0x00,
				0xff, 0xff, 0xff, 0x7f, //i32
				0x00,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f, //i64
				0x00,
				0xff, //u8
				0x00,
				0xff, 0xff, //u16
				0x00,
				0xff, 0xff, 0xff, 0xff, //u32
				0x00,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, //u64
				0x00,
				0xff, 0xff, 0x7f, 0x7f, //f32
				0x00,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xef, 0x7f, //f64
				0x00, 0x00, 0x00, 0x00,
				0x03, 0x00, //binary
				0x41, 0x42, 0x43,
				0x00, 0x00, 0x00, 0x00,
				0x10, 0x00, //nchar
				0x9b, 0x6d, 0x00, 0x00, 0x1d, 0x60, 0x00, 0x00, 0x70, 0x65, 0x00, 0x00, 0x6e, 0x63, 0x00, 0x00,
			},
			wantErr: false,
		},
		{
			name: "all with nil",
			args: args{
				params: []*param.Param{
					param.NewParam(3).AddTimestamp(time.Unix(1666248065, 0), 0).AddNull().AddTimestamp(time.Unix(1666248067, 0), 0),
					param.NewParam(3).AddBool(true).AddNull().AddBool(true),
					param.NewParam(3).AddTinyint(1).AddNull().AddTinyint(1),
					param.NewParam(3).AddSmallint(1).AddNull().AddSmallint(1),
					param.NewParam(3).AddInt(1).AddNull().AddInt(1),
					param.NewParam(3).AddBigint(1).AddNull().AddBigint(1),
					param.NewParam(3).AddUTinyint(1).AddNull().AddUTinyint(1),
					param.NewParam(3).AddUSmallint(1).AddNull().AddUSmallint(1),
					param.NewParam(3).AddUInt(1).AddNull().AddUInt(1),
					param.NewParam(3).AddUBigint(1).AddNull().AddUBigint(1),
					param.NewParam(3).AddFloat(1).AddNull().AddFloat(1),
					param.NewParam(3).AddDouble(1).AddNull().AddDouble(1),
					param.NewParam(3).AddBinary([]byte("test_binary")).AddNull().AddBinary([]byte("test_binary")),
					param.NewParam(3).AddNchar("test_nchar").AddNull().AddNchar("test_nchar"),
					param.NewParam(3).AddJson([]byte("{\"a\":1}")).AddNull().AddJson([]byte("{\"a\":1}")),
				},
				colType: param.NewColumnType(15).
					AddTimestamp().
					AddBool().
					AddTinyint().
					AddSmallint().
					AddInt().
					AddBigint().
					AddUTinyint().
					AddUSmallint().
					AddUInt().
					AddUBigint().
					AddFloat().
					AddDouble().
					AddBinary(0).
					AddNchar(0).
					AddJson(0),
			},
			want: []byte{
				0x01, 0x00, 0x00, 0x00,
				0xec, 0x01, 0x00, 0x00,
				0x03, 0x00, 0x00, 0x00,
				0x0f, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				//types
				0x09, 0x08, 0x00, 0x00, 0x00,
				0x01, 0x01, 0x00, 0x00, 0x00,
				0x02, 0x01, 0x00, 0x00, 0x00,
				0x03, 0x02, 0x00, 0x00, 0x00,
				0x04, 0x04, 0x00, 0x00, 0x00,
				0x05, 0x08, 0x00, 0x00, 0x00,
				0x0b, 0x01, 0x00, 0x00, 0x00,
				0x0c, 0x02, 0x00, 0x00, 0x00,
				0x0d, 0x04, 0x00, 0x00, 0x00,
				0x0e, 0x08, 0x00, 0x00, 0x00,
				0x06, 0x04, 0x00, 0x00, 0x00,
				0x07, 0x08, 0x00, 0x00, 0x00,
				0x08, 0x00, 0x00, 0x00, 0x00,
				0x0a, 0x00, 0x00, 0x00, 0x00,
				0x0f, 0x00, 0x00, 0x00, 0x00,
				//lengths
				0x18, 0x00, 0x00, 0x00,
				0x03, 0x00, 0x00, 0x00,
				0x03, 0x00, 0x00, 0x00,
				0x06, 0x00, 0x00, 0x00,
				0x0c, 0x00, 0x00, 0x00,
				0x18, 0x00, 0x00, 0x00,
				0x03, 0x00, 0x00, 0x00,
				0x06, 0x00, 0x00, 0x00,
				0x0c, 0x00, 0x00, 0x00,
				0x18, 0x00, 0x00, 0x00,
				0x0c, 0x00, 0x00, 0x00,
				0x18, 0x00, 0x00, 0x00,
				0x1a, 0x00, 0x00, 0x00,
				0x54, 0x00, 0x00, 0x00,
				0x12, 0x00, 0x00, 0x00,
				// ts
				0x40,
				0xe8, 0xbf, 0x1f, 0xf4, 0x83, 0x01, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0xb8, 0xc7, 0x1f, 0xf4, 0x83, 0x01, 0x00, 0x00,

				// bool
				0x40,
				0x01,
				0x00,
				0x01,

				// i8
				0x40,
				0x01,
				0x00,
				0x01,

				//int16
				0x40,
				0x01, 0x00,
				0x00, 0x00,
				0x01, 0x00,

				//int32
				0x40,
				0x01, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x01, 0x00, 0x00, 0x00,

				//int64
				0x40,
				0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,

				//uint8
				0x40,
				0x01,
				0x00,
				0x01,

				//uint16
				0x40,
				0x01, 0x00,
				0x00, 0x00,
				0x01, 0x00,

				//uint32
				0x40,
				0x01, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x01, 0x00, 0x00, 0x00,

				//uint64
				0x40,
				0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,

				//float
				0x40,
				0x00, 0x00, 0x80, 0x3f,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x80, 0x3f,

				//double
				0x40,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xf0, 0x3f,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xf0, 0x3f,

				//binary
				0x00, 0x00, 0x00, 0x00,
				0xff, 0xff, 0xff, 0xff,
				0x0d, 0x00, 0x00, 0x00,
				0x0b, 0x00,
				0x74, 0x65, 0x73, 0x74, 0x5f, 0x62, 0x69, 0x6e, 0x61, 0x72, 0x79,
				0x0b, 0x00,
				0x74, 0x65, 0x73, 0x74, 0x5f, 0x62, 0x69, 0x6e, 0x61, 0x72, 0x79,

				//nchar
				0x00, 0x00, 0x00, 0x00,
				0xff, 0xff, 0xff, 0xff,
				0x2a, 0x00, 0x00, 0x00,
				0x28, 0x00,
				0x74, 0x00, 0x00, 0x00, 0x65, 0x00, 0x00, 0x00, 0x73, 0x00,
				0x00, 0x00, 0x74, 0x00, 0x00, 0x00, 0x5f, 0x00, 0x00, 0x00,
				0x6e, 0x00, 0x00, 0x00, 0x63, 0x00, 0x00, 0x00, 0x68, 0x00,
				0x00, 0x00, 0x61, 0x00, 0x00, 0x00, 0x72, 0x00, 0x00, 0x00,
				0x28, 0x00,
				0x74, 0x00, 0x00, 0x00, 0x65, 0x00, 0x00, 0x00, 0x73, 0x00,
				0x00, 0x00, 0x74, 0x00, 0x00, 0x00, 0x5f, 0x00, 0x00, 0x00,
				0x6e, 0x00, 0x00, 0x00, 0x63, 0x00, 0x00, 0x00, 0x68, 0x00,
				0x00, 0x00, 0x61, 0x00, 0x00, 0x00, 0x72, 0x00, 0x00, 0x00,

				//json
				0x00, 0x00, 0x00, 0x00,
				0xff, 0xff, 0xff, 0xff,
				0x09, 0x00, 0x00, 0x00,
				0x07, 0x00,
				0x7b, 0x22, 0x61, 0x22, 0x3a, 0x31, 0x7d,
				0x07, 0x00,
				0x7b, 0x22, 0x61, 0x22, 0x3a, 0x31, 0x7d,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SerializeRawBlock(tt.args.params, tt.args.colType)
			if (err != nil) != tt.wantErr {
				t.Errorf("SerializeRawBlock() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SerializeRawBlock() got = %v, want %v", got, tt.want)
			}
		})
	}
}
