package artifact

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"
)

const (
	accPublic    = 0x0001
	accPrivate   = 0x0002
	accProtected = 0x0004
	accStatic    = 0x0008
	accFinal     = 0x0010
	accInterface = 0x0200
	accAbstract  = 0x0400
	accSynthetic = 0x1000
)

type classReader struct {
	data []byte
	pos  int
}

func (r *classReader) u1() (uint8, error) {
	if r.pos+1 > len(r.data) {
		return 0, fmt.Errorf("unexpected end of class file")
	}
	value := r.data[r.pos]
	r.pos++
	return value, nil
}

func (r *classReader) u2() (uint16, error) {
	if r.pos+2 > len(r.data) {
		return 0, fmt.Errorf("unexpected end of class file")
	}
	value := binary.BigEndian.Uint16(r.data[r.pos : r.pos+2])
	r.pos += 2
	return value, nil
}

func (r *classReader) u4() (uint32, error) {
	if r.pos+4 > len(r.data) {
		return 0, fmt.Errorf("unexpected end of class file")
	}
	value := binary.BigEndian.Uint32(r.data[r.pos : r.pos+4])
	r.pos += 4
	return value, nil
}

func (r *classReader) bytes(length uint32) ([]byte, error) {
	if uint64(r.pos)+uint64(length) > uint64(len(r.data)) {
		return nil, fmt.Errorf("unexpected end of class file")
	}
	value := r.data[r.pos : r.pos+int(length)]
	r.pos += int(length)
	return value, nil
}

type cpEntry struct {
	tag   uint8
	text  string
	index uint16
	raw   uint64
}

type classPool []cpEntry

func (pool classPool) utf8(index uint16) (string, error) {
	if index == 0 || int(index) >= len(pool) || pool[index].tag != 1 {
		return "", fmt.Errorf("invalid UTF-8 constant-pool index %d", index)
	}
	return pool[index].text, nil
}

func (pool classPool) className(index uint16) (string, error) {
	if index == 0 {
		return "", nil
	}
	if int(index) >= len(pool) || pool[index].tag != 7 {
		return "", fmt.Errorf("invalid class constant-pool index %d", index)
	}
	return pool.utf8(pool[index].index)
}

func parseClassFile(data []byte) (Class, bool, error) {
	reader := &classReader{data: data}
	magic, err := reader.u4()
	if err != nil || magic != 0xcafebabe {
		return Class{}, false, fmt.Errorf("invalid class-file magic")
	}
	if _, err := reader.u2(); err != nil {
		return Class{}, false, err
	}
	if _, err := reader.u2(); err != nil {
		return Class{}, false, err
	}
	pool, kotlinMetadata, err := readConstantPool(reader)
	if err != nil {
		return Class{}, false, err
	}
	access, err := reader.u2()
	if err != nil {
		return Class{}, false, err
	}
	thisClass, err := reader.u2()
	if err != nil {
		return Class{}, false, err
	}
	superClass, err := reader.u2()
	if err != nil {
		return Class{}, false, err
	}
	name, err := pool.className(thisClass)
	if err != nil {
		return Class{}, false, err
	}
	superName, err := pool.className(superClass)
	if err != nil {
		return Class{}, false, err
	}
	class := Class{Name: name, Super: superName, Access: access, KotlinMetadata: kotlinMetadata}

	interfaceCount, err := reader.u2()
	if err != nil {
		return Class{}, false, err
	}
	for index := 0; index < int(interfaceCount); index++ {
		interfaceIndex, readErr := reader.u2()
		if readErr != nil {
			return Class{}, false, readErr
		}
		interfaceName, nameErr := pool.className(interfaceIndex)
		if nameErr != nil {
			return Class{}, false, nameErr
		}
		class.Interfaces = append(class.Interfaces, interfaceName)
	}
	sort.Strings(class.Interfaces)

	fields, err := readMembers(reader, pool, "field")
	if err != nil {
		return Class{}, false, err
	}
	methods, err := readMembers(reader, pool, "method")
	if err != nil {
		return Class{}, false, err
	}
	class.Members = append(fields, methods...)
	sort.Slice(class.Members, func(i, j int) bool { return class.Members[i].Key() < class.Members[j].Key() })

	attributes, err := readAttributes(reader, pool)
	if err != nil {
		return Class{}, false, err
	}
	class.Signature = attributes.signature
	class.PermittedSubclasses = attributes.permittedSubclasses
	if reader.pos != len(data) {
		return Class{}, false, fmt.Errorf("class file has %d trailing bytes", len(data)-reader.pos)
	}
	return class, isAPI(access), nil
}

func readConstantPool(reader *classReader) (classPool, bool, error) {
	count, err := reader.u2()
	if err != nil {
		return nil, false, err
	}
	pool := make(classPool, count)
	kotlinMetadata := false
	for index := 1; index < int(count); index++ {
		tag, readErr := reader.u1()
		if readErr != nil {
			return nil, false, readErr
		}
		entry := cpEntry{tag: tag}
		switch tag {
		case 1:
			length, lengthErr := reader.u2()
			if lengthErr != nil {
				return nil, false, lengthErr
			}
			value, valueErr := reader.bytes(uint32(length))
			if valueErr != nil {
				return nil, false, valueErr
			}
			entry.text = string(value)
			if entry.text == "Lkotlin/Metadata;" {
				kotlinMetadata = true
			}
		case 3, 4:
			value, valueErr := reader.u4()
			if valueErr != nil {
				return nil, false, valueErr
			}
			entry.raw = uint64(value)
		case 5, 6:
			if index+1 >= int(count) {
				return nil, false, fmt.Errorf("two-slot constant-pool entry %d has no reserved slot", index)
			}
			high, valueErr := reader.u4()
			if valueErr != nil {
				return nil, false, valueErr
			}
			low, valueErr := reader.u4()
			if valueErr != nil {
				return nil, false, valueErr
			}
			entry.raw = uint64(high)<<32 | uint64(low)
			pool[index] = entry
			index++
			continue
		case 7, 8, 16, 19, 20:
			value, valueErr := reader.u2()
			if valueErr != nil {
				return nil, false, valueErr
			}
			entry.index = value
		case 9, 10, 11, 12, 17, 18:
			if _, valueErr := reader.u2(); valueErr != nil {
				return nil, false, valueErr
			}
			if _, valueErr := reader.u2(); valueErr != nil {
				return nil, false, valueErr
			}
		case 15:
			if _, valueErr := reader.u1(); valueErr != nil {
				return nil, false, valueErr
			}
			if _, valueErr := reader.u2(); valueErr != nil {
				return nil, false, valueErr
			}
		default:
			return nil, false, fmt.Errorf("unsupported constant-pool tag %d", tag)
		}
		pool[index] = entry
	}
	return pool, kotlinMetadata, nil
}

type parsedAttributes struct {
	signature           string
	constant            string
	exceptions          []string
	permittedSubclasses []string
}

func readMembers(reader *classReader, pool classPool, kind string) ([]Member, error) {
	count, err := reader.u2()
	if err != nil {
		return nil, err
	}
	var members []Member
	for index := 0; index < int(count); index++ {
		access, readErr := reader.u2()
		if readErr != nil {
			return nil, readErr
		}
		nameIndex, readErr := reader.u2()
		if readErr != nil {
			return nil, readErr
		}
		descriptorIndex, readErr := reader.u2()
		if readErr != nil {
			return nil, readErr
		}
		name, nameErr := pool.utf8(nameIndex)
		if nameErr != nil {
			return nil, nameErr
		}
		descriptor, descriptorErr := pool.utf8(descriptorIndex)
		if descriptorErr != nil {
			return nil, descriptorErr
		}
		attributes, attributesErr := readAttributes(reader, pool)
		if attributesErr != nil {
			return nil, attributesErr
		}
		if !isAPI(access) || name == "<clinit>" {
			continue
		}
		members = append(members, Member{
			Kind: kind, Name: name, Descriptor: descriptor, Signature: attributes.signature,
			Access: access, Constant: attributes.constant, Exceptions: attributes.exceptions,
		})
	}
	return members, nil
}

func readAttributes(reader *classReader, pool classPool) (parsedAttributes, error) {
	count, err := reader.u2()
	if err != nil {
		return parsedAttributes{}, err
	}
	var result parsedAttributes
	for index := 0; index < int(count); index++ {
		nameIndex, readErr := reader.u2()
		if readErr != nil {
			return parsedAttributes{}, readErr
		}
		length, readErr := reader.u4()
		if readErr != nil {
			return parsedAttributes{}, readErr
		}
		name, nameErr := pool.utf8(nameIndex)
		if nameErr != nil {
			return parsedAttributes{}, nameErr
		}
		data, dataErr := reader.bytes(length)
		if dataErr != nil {
			return parsedAttributes{}, dataErr
		}
		attributeReader := &classReader{data: data}
		switch name {
		case "Signature":
			valueIndex, valueErr := attributeReader.u2()
			if valueErr != nil {
				return parsedAttributes{}, valueErr
			}
			result.signature, valueErr = pool.utf8(valueIndex)
			if valueErr != nil {
				return parsedAttributes{}, valueErr
			}
		case "ConstantValue":
			valueIndex, valueErr := attributeReader.u2()
			if valueErr != nil {
				return parsedAttributes{}, valueErr
			}
			result.constant = constantValue(pool, valueIndex)
		case "Exceptions":
			exceptionCount, valueErr := attributeReader.u2()
			if valueErr != nil {
				return parsedAttributes{}, valueErr
			}
			for exceptionIndex := 0; exceptionIndex < int(exceptionCount); exceptionIndex++ {
				classIndex, indexErr := attributeReader.u2()
				if indexErr != nil {
					return parsedAttributes{}, indexErr
				}
				className, classErr := pool.className(classIndex)
				if classErr != nil {
					return parsedAttributes{}, classErr
				}
				result.exceptions = append(result.exceptions, className)
			}
			sort.Strings(result.exceptions)
		case "PermittedSubclasses":
			classCount, valueErr := attributeReader.u2()
			if valueErr != nil {
				return parsedAttributes{}, valueErr
			}
			for classOffset := 0; classOffset < int(classCount); classOffset++ {
				classIndex, indexErr := attributeReader.u2()
				if indexErr != nil {
					return parsedAttributes{}, indexErr
				}
				className, classErr := pool.className(classIndex)
				if classErr != nil {
					return parsedAttributes{}, classErr
				}
				result.permittedSubclasses = append(result.permittedSubclasses, className)
			}
			sort.Strings(result.permittedSubclasses)
		}
	}
	return result, nil
}

func constantValue(pool classPool, index uint16) string {
	if index == 0 || int(index) >= len(pool) {
		return ""
	}
	entry := pool[index]
	switch entry.tag {
	case 1:
		return entry.text
	case 3:
		return fmt.Sprintf("int:%d", int32(entry.raw))
	case 4:
		return fmt.Sprintf("float:%g", math.Float32frombits(uint32(entry.raw)))
	case 5:
		return fmt.Sprintf("long:%d", int64(entry.raw))
	case 6:
		return fmt.Sprintf("double:%g", math.Float64frombits(entry.raw))
	case 8:
		value, _ := pool.utf8(entry.index)
		return "string:" + value
	default:
		return ""
	}
}

func isAPI(access uint16) bool {
	return access&accPublic != 0 || access&accProtected != 0
}
