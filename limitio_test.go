package limitio

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestAtMostFirstBytes(t *testing.T) {
	tests := []struct {
		name       string
		give       []byte
		sizeAtMost int
		want       []byte
	}{
		{
			"nil",
			nil,
			50,
			nil,
		},
		{
			"empty",
			[]byte{},
			50,
			[]byte{},
		},
		{
			"zero",
			[]byte("hello world!"),
			0,
			[]byte{},
		},
		{
			"some",
			[]byte("hello world!"),
			7,
			[]byte("hello w"),
		},
		{
			"exactly",
			[]byte("hello world!"),
			13,
			[]byte("hello world!"),
		},
		{
			"more",
			[]byte("hello world!"),
			14,
			[]byte("hello world!"),
		},
		{
			"quite more",
			[]byte("hello world!"),
			100,
			[]byte("hello world!"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AtMostFirstNBytes(tt.give, tt.sizeAtMost); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AtMostFirstNBytes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLimitWriter_Write(t *testing.T) {
	const (
		corpse    = "hello world!"
		corpseLen = len(corpse)
	)

	tests := []struct {
		name                 string
		limit                int
		regardOverSizeNormal bool
		giveMsg              []byte
		wantMsg              []byte
		wantN                int
		wantErr              bool
	}{
		{
			name:                 "nil zero",
			limit:                0,
			regardOverSizeNormal: true,
			giveMsg:              nil,
			wantMsg:              nil,
			wantN:                0,
			wantErr:              false,
		},
		{
			name:                 "nil more",
			limit:                1,
			regardOverSizeNormal: false,
			giveMsg:              nil,
			wantMsg:              nil,
			wantN:                0,
			wantErr:              false,
		},
		{
			name:                 "not oversize",
			limit:                corpseLen + 1,
			regardOverSizeNormal: false,
			giveMsg:              []byte(corpse),
			wantMsg:              []byte(corpse),
			wantN:                corpseLen,
			wantErr:              false,
		},
		{
			name:                 "not oversize but regardOverSizeNormal",
			limit:                corpseLen + 1,
			regardOverSizeNormal: true,
			giveMsg:              []byte(corpse),
			wantMsg:              []byte(corpse),
			wantN:                corpseLen,
			wantErr:              false,
		},
		{
			name:                 "exact",
			limit:                corpseLen,
			regardOverSizeNormal: false,
			giveMsg:              []byte(corpse),
			wantMsg:              []byte(corpse),
			wantN:                corpseLen,
			wantErr:              false,
		},
		{
			name:                 "exact but regardOverSizeNormal",
			limit:                corpseLen,
			regardOverSizeNormal: true,
			giveMsg:              []byte(corpse),
			wantMsg:              []byte(corpse),
			wantN:                corpseLen,
			wantErr:              false,
		},
		{
			name:                 "more",
			limit:                corpseLen - 1,
			regardOverSizeNormal: false,
			giveMsg:              []byte(corpse),
			wantMsg:              []byte(corpse[:corpseLen-1]),
			wantN:                corpseLen - 1,
			wantErr:              true,
		},
		{
			name:                 "more but regardOverSizeNormal",
			limit:                corpseLen - 1,
			regardOverSizeNormal: true,
			giveMsg:              []byte(corpse),
			wantMsg:              []byte(corpse[:corpseLen-1]),
			wantN:                corpseLen,
			wantErr:              false,
		},
		{
			name:                 "a lot",
			limit:                3,
			regardOverSizeNormal: false,
			giveMsg:              []byte(corpse),
			wantMsg:              []byte(corpse[:3]),
			wantN:                3,
			wantErr:              true,
		},
		{
			name:                 "a lot but regardOverSizeNormal",
			limit:                3,
			regardOverSizeNormal: true,
			giveMsg:              []byte(corpse),
			wantMsg:              []byte(corpse[:3]),
			wantN:                corpseLen,
			wantErr:              false,
		},
	}
	for _, tt := range tests {
		// one stage write
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			lw := NewWriter(&buf, tt.limit, tt.regardOverSizeNormal)
			gotN, err := lw.Write(tt.giveMsg)
			if err != nil && !errors.Is(err, ErrThresholdExceeded) {
				t.Errorf("err = %s, but does not come from ErrThresholdExceeded", err)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("Write() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotN != tt.wantN {
				t.Errorf("Write() gotN = %v, want %v", gotN, tt.wantN)
			}
			if gotMsg := buf.Bytes(); !reflect.DeepEqual(gotMsg, tt.wantMsg) {
				t.Errorf("got msg = %v, want %v", gotMsg, tt.wantMsg)
			}
		})

		// two stage write
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.giveMsg) <= 1 {
				return
			}
			var buf bytes.Buffer
			lw := NewWriter(&buf, tt.limit, tt.regardOverSizeNormal)

			gotN1, err := lw.Write(tt.giveMsg[:1])
			if err != nil {
				t.Errorf("writing first chunk: %s", err)
				return
			}

			gotN2, err := lw.Write(tt.giveMsg[1:])
			if err != nil && !errors.Is(err, ErrThresholdExceeded) {
				t.Errorf("err = %s, but does not come from ErrThresholdExceeded", err)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("Write() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			gotN := gotN1 + gotN2
			if gotN != tt.wantN {
				t.Errorf("Write() gotN = %v, want %v", gotN, tt.wantN)
			}
			if gotMsg := buf.Bytes(); !reflect.DeepEqual(gotMsg, tt.wantMsg) {
				t.Errorf("got msg = %v, want %v", gotMsg, tt.wantMsg)
			}
		})
	}
}

func TestReader_Read(t *testing.T) {
	const (
		corpse    = "hello world!"
		corpseLen = len(corpse)
	)

	type fields struct {
		limit             int
		regardOverSizeEOF bool
	}
	tests := []struct {
		name    string
		fields  fields
		giveN   int
		wantN   int
		wantP   string
		wantErr bool
	}{
		{
			name: "zero limit",
			fields: fields{
				limit:             0,
				regardOverSizeEOF: false,
			},
			giveN:   corpseLen,
			wantN:   0,
			wantP:   "",
			wantErr: true,
		},
		{
			name: "zero limit EOF",
			fields: fields{
				limit:             0,
				regardOverSizeEOF: true,
			},
			giveN:   corpseLen,
			wantN:   0,
			wantP:   "",
			wantErr: true,
		},
		{
			name: "one limit",
			fields: fields{
				limit:             1,
				regardOverSizeEOF: false,
			},
			giveN:   corpseLen,
			wantN:   1,
			wantP:   "h",
			wantErr: true,
		},
		{
			name: "one limit EOF",
			fields: fields{
				limit:             1,
				regardOverSizeEOF: true,
			},
			giveN:   corpseLen,
			wantN:   1,
			wantP:   "h",
			wantErr: true,
		},
		{
			name: "exact",
			fields: fields{
				limit:             corpseLen,
				regardOverSizeEOF: false,
			},
			giveN:   corpseLen,
			wantN:   corpseLen,
			wantP:   corpse,
			wantErr: false,
		},
		{
			name: "exact EOF",
			fields: fields{
				limit:             corpseLen,
				regardOverSizeEOF: true,
			},
			giveN:   corpseLen,
			wantN:   corpseLen,
			wantP:   corpse,
			wantErr: false,
		},
		{
			name: "more",
			fields: fields{
				limit:             corpseLen + 1,
				regardOverSizeEOF: false,
			},
			giveN:   corpseLen,
			wantN:   corpseLen,
			wantP:   corpse,
			wantErr: false,
		},
		{
			name: "more EOF",
			fields: fields{
				limit:             corpseLen + 1,
				regardOverSizeEOF: true,
			},
			giveN:   corpseLen,
			wantN:   corpseLen,
			wantP:   corpse,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				innerReader = strings.NewReader(corpse[:tt.giveN])
				gotP        = make([]byte, corpseLen)
			)

			lr := NewReader(innerReader, tt.fields.limit, tt.fields.regardOverSizeEOF)
			gotN, err := lr.Read(gotP)
			if tt.wantErr && err == nil {
				var gotN2 int
				gotN2, err = lr.Read(gotP)
				if gotN2 != 0 {
					t.Errorf("gotN2 = %d, want 0", gotN2)
				}
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("Read() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				if tt.fields.regardOverSizeEOF {
					if err != io.EOF {
						t.Errorf("err = %s, want EOF", err)
					}
				} else {
					if !errors.Is(err, ErrThresholdExceeded) {
						t.Errorf("err = %s, does not coming from ErrThresholdExceeded", err)
					}
				}
			}
			if gotN != tt.wantN {
				t.Errorf("Read() gotN = %v, want %v", gotN, tt.wantN)
			}
			if string(bytes.TrimRight(gotP, string(rune(0)))) != tt.wantP {
				t.Errorf("gotP = %s, want %s", gotP, tt.wantP)
			}
		})
	}
}
