package storage

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"time"
)

type bsonDoc map[string]any

type bsonArray []any

func encodeBSON(doc bsonDoc) ([]byte, error) {
	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.LittleEndian, int32(0)) // placeholder length
	keys := make([]string, 0, len(doc))
	for k := range doc {
		keys = append(keys, k)
	}
	// Orden estable para facilitar pruebas; MongoDB no exige orden salvo que algunos comandos
	// prefieran el nombre del comando primero. Por ello insert/find se construyen en bsonOrdered.
	sort.Strings(keys)
	for _, k := range keys {
		if err := writeBSONElement(buf, k, doc[k]); err != nil {
			return nil, err
		}
	}
	buf.WriteByte(0)
	b := buf.Bytes()
	binary.LittleEndian.PutUint32(b[:4], uint32(len(b)))
	return b, nil
}

type bsonElement struct {
	Key   string
	Value any
}

func encodeBSONOrdered(elements []bsonElement) ([]byte, error) {
	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.LittleEndian, int32(0))
	for _, e := range elements {
		if err := writeBSONElement(buf, e.Key, e.Value); err != nil {
			return nil, err
		}
	}
	buf.WriteByte(0)
	b := buf.Bytes()
	binary.LittleEndian.PutUint32(b[:4], uint32(len(b)))
	return b, nil
}

func writeCString(buf *bytes.Buffer, s string) {
	buf.WriteString(s)
	buf.WriteByte(0)
}

func writeBSONElement(buf *bytes.Buffer, key string, value any) error {
	switch v := value.(type) {
	case string:
		buf.WriteByte(0x02)
		writeCString(buf, key)
		_ = binary.Write(buf, binary.LittleEndian, int32(len(v)+1))
		buf.WriteString(v)
		buf.WriteByte(0)
	case int:
		buf.WriteByte(0x10)
		writeCString(buf, key)
		_ = binary.Write(buf, binary.LittleEndian, int32(v))
	case int32:
		buf.WriteByte(0x10)
		writeCString(buf, key)
		_ = binary.Write(buf, binary.LittleEndian, v)
	case int64:
		buf.WriteByte(0x12)
		writeCString(buf, key)
		_ = binary.Write(buf, binary.LittleEndian, v)
	case float64:
		buf.WriteByte(0x01)
		writeCString(buf, key)
		_ = binary.Write(buf, binary.LittleEndian, math.Float64bits(v))
	case bool:
		buf.WriteByte(0x08)
		writeCString(buf, key)
		if v {
			buf.WriteByte(1)
		} else {
			buf.WriteByte(0)
		}
	case time.Time:
		buf.WriteByte(0x09)
		writeCString(buf, key)
		_ = binary.Write(buf, binary.LittleEndian, v.UnixMilli())
	case bsonDoc:
		b, err := encodeBSON(v)
		if err != nil {
			return err
		}
		buf.WriteByte(0x03)
		writeCString(buf, key)
		buf.Write(b)
	case []bsonElement:
		b, err := encodeBSONOrdered(v)
		if err != nil {
			return err
		}
		buf.WriteByte(0x03)
		writeCString(buf, key)
		buf.Write(b)
	case bsonArray:
		b, err := encodeBSONArray(v)
		if err != nil {
			return err
		}
		buf.WriteByte(0x04)
		writeCString(buf, key)
		buf.Write(b)
	case []bsonDoc:
		arr := make(bsonArray, 0, len(v))
		for _, item := range v {
			arr = append(arr, item)
		}
		b, err := encodeBSONArray(arr)
		if err != nil {
			return err
		}
		buf.WriteByte(0x04)
		writeCString(buf, key)
		buf.Write(b)
	case nil:
		buf.WriteByte(0x0A)
		writeCString(buf, key)
	default:
		return fmt.Errorf("tipo BSON no soportado para %s: %T", key, value)
	}
	return nil
}

func encodeBSONArray(arr bsonArray) ([]byte, error) {
	elements := make([]bsonElement, 0, len(arr))
	for i, v := range arr {
		elements = append(elements, bsonElement{Key: fmt.Sprintf("%d", i), Value: v})
	}
	return encodeBSONOrdered(elements)
}

func decodeBSON(b []byte) (bsonDoc, error) {
	if len(b) < 5 {
		return nil, fmt.Errorf("BSON demasiado corto")
	}
	length := int(int32(binary.LittleEndian.Uint32(b[:4])))
	if length <= 0 || length > len(b) {
		return nil, fmt.Errorf("longitud BSON invalida: %d", length)
	}
	pos := 4
	doc := bsonDoc{}
	for pos < length-1 {
		t := b[pos]
		pos++
		keyStart := pos
		for pos < length && b[pos] != 0 {
			pos++
		}
		if pos >= length {
			return nil, fmt.Errorf("cstring BSON incompleto")
		}
		key := string(b[keyStart:pos])
		pos++ // null
		value, next, err := readBSONValue(t, b, pos)
		if err != nil {
			return nil, err
		}
		doc[key] = value
		pos = next
	}
	return doc, nil
}

func readBSONValue(t byte, b []byte, pos int) (any, int, error) {
	switch t {
	case 0x01: // double
		if pos+8 > len(b) {
			return nil, pos, fmt.Errorf("double incompleto")
		}
		return math.Float64frombits(binary.LittleEndian.Uint64(b[pos : pos+8])), pos + 8, nil
	case 0x02: // string
		if pos+4 > len(b) {
			return nil, pos, fmt.Errorf("string length incompleto")
		}
		l := int(int32(binary.LittleEndian.Uint32(b[pos : pos+4])))
		pos += 4
		if l <= 0 || pos+l > len(b) {
			return nil, pos, fmt.Errorf("string BSON invalido")
		}
		return string(b[pos : pos+l-1]), pos + l, nil
	case 0x03: // doc
		if pos+4 > len(b) {
			return nil, pos, fmt.Errorf("doc incompleto")
		}
		l := int(int32(binary.LittleEndian.Uint32(b[pos : pos+4])))
		if l <= 0 || pos+l > len(b) {
			return nil, pos, fmt.Errorf("doc BSON invalido")
		}
		d, err := decodeBSON(b[pos : pos+l])
		return d, pos + l, err
	case 0x04: // array
		if pos+4 > len(b) {
			return nil, pos, fmt.Errorf("array incompleto")
		}
		l := int(int32(binary.LittleEndian.Uint32(b[pos : pos+4])))
		if l <= 0 || pos+l > len(b) {
			return nil, pos, fmt.Errorf("array BSON invalido")
		}
		d, err := decodeBSON(b[pos : pos+l])
		if err != nil {
			return nil, pos, err
		}
		arr := bsonArray{}
		for i := 0; ; i++ {
			key := fmt.Sprintf("%d", i)
			v, ok := d[key]
			if !ok {
				break
			}
			arr = append(arr, v)
		}
		return arr, pos + l, nil
	case 0x08: // bool
		if pos+1 > len(b) {
			return nil, pos, fmt.Errorf("bool incompleto")
		}
		return b[pos] == 1, pos + 1, nil
	case 0x07: // ObjectID generado automaticamente por MongoDB (_id)
		if pos+12 > len(b) {
			return nil, pos, fmt.Errorf("objectID incompleto")
		}
		// No se usa para la logica PC4; se conserva como hexadecimal para poder
		// leer documentos devueltos por find sin fallar al encontrar el campo _id.
		return fmt.Sprintf("%x", b[pos:pos+12]), pos + 12, nil
	case 0x09: // datetime
		if pos+8 > len(b) {
			return nil, pos, fmt.Errorf("date incompleto")
		}
		ms := int64(binary.LittleEndian.Uint64(b[pos : pos+8]))
		return time.UnixMilli(ms).UTC(), pos + 8, nil
	case 0x0A:
		return nil, pos, nil
	case 0x10: // int32
		if pos+4 > len(b) {
			return nil, pos, fmt.Errorf("int32 incompleto")
		}
		return int(int32(binary.LittleEndian.Uint32(b[pos : pos+4]))), pos + 4, nil
	case 0x12: // int64
		if pos+8 > len(b) {
			return nil, pos, fmt.Errorf("int64 incompleto")
		}
		return int64(binary.LittleEndian.Uint64(b[pos : pos+8])), pos + 8, nil
	default:
		return nil, pos, fmt.Errorf("tipo BSON no soportado al leer: 0x%x", t)
	}
}
