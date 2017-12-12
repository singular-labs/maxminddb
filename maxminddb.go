// see http://maxmind.github.io/MaxMind-DB/
// for the MaxMindDB format specification
package maxminddb

import (
	"bufio"
	"bytes"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"math"
	"math/bits"
	"net"
	"os"
	"time"
)

type Node struct {
	Bit0 int
	Bit1 int
}

type GeoName struct {
	Country_ISO string
	Country     string
	Region      string
	City        string
	Latitude    float32
	Longitude   float32
}

var (
	nodes    []Node         = []Node{Node{}}
	node_seq uint32         = 1
	data     *bytes.Buffer  = new(bytes.Buffer)
	terms    map[string]int = make(map[string]int, 100000)
)

func IP_to_uint32(ip net.IP) uint32 {
	ip4 := ip.To4()
	if ip4 == nil {
		log.Fatal("not IPv4", ip)
	}
	return uint32(ip4[0])<<24 + uint32(ip4[1])<<16 + uint32(ip4[2])<<8 + uint32(ip4[3])
}

func Uint32_to_IP(i uint32) net.IP {
	return net.IPv4(byte(i>>24&0xFF), byte(i>>16&0xFF), byte(i>>8&0xFF), byte(i&0xFF))
}

func new_node() int {
	node_seq++
	// > 0:  node index (1-based)
	// == 0: nil
	// < 0: geonode id (1-based) * -1
	nodes = append(nodes, Node{Bit0: 0, Bit1: 0})
	return int(node_seq)
}

func range_to_subnets(a uint32, b uint32) []net.IPNet {
	out := make([]net.IPNet, 0)
	for a <= b {
		subnet := uint(bits.TrailingZeros32(a))
		for a+(uint32(1)<<subnet)-1 > b {
			subnet--
		}
		out = append(out, net.IPNet{Uint32_to_IP(a), net.CIDRMask(int(32-subnet), 32)})
		new_a := a + (uint32(1) << subnet)
		// wrap-around
		if new_a < a {
			break
		}
		a = new_a
	}

	return out
}

func Push_Range(begin net.IP, end net.IP, geo GeoName) {
	for _, subnet := range range_to_subnets(IP_to_uint32(begin), IP_to_uint32(end)) {
		Push(subnet, geo)
	}
}

func Push(subnet net.IPNet, geo GeoName) {
	ones, bits := subnet.Mask.Size()
	netip := IP_to_uint32(subnet.IP)
	current := 0
	for i := 0; i < ones-1; i++ {
		if netip&(uint32(1)<<uint(bits-i-1)) == 0 {
			if nodes[current].Bit0 == 0 {
				nodes[current].Bit0 = new_node()
			}
			current = nodes[current].Bit0 - 1
		} else {
			if nodes[current].Bit1 == 0 {
				nodes[current].Bit1 = new_node()
			}
			current = nodes[current].Bit1 - 1
		}
	}
	if current < 0 {
		log.Fatal("tried to push a geo node ", subnet.String(), " ", geo, " under another ", current)
	}
	val := -intern_geo(geo) - 1
	if netip&(uint32(1)<<uint(bits-ones)) == 0 {
		// substract -1 so offset=0 is not confused with "no data"
		nodes[current].Bit0 = val
	} else {
		// substract -1 so offset=0 is not confused with "no data"
		nodes[current].Bit1 = val
	}
}

func write_utf8string(out io.Writer, s string) {
	l := len(s)
	switch {
	case l < 29:
		out.Write([]byte{byte(0x40 | l)})
		io.WriteString(out, s)
	case l < 29+255:
		out.Write([]byte{0x5d, byte(l - 29)})
		io.WriteString(out, s)
	case l < 285+65535:
		l = l - 285
		out.Write([]byte{0x5e, byte((l & 0xFF00) >> 8), byte(l & 0xFF)})
		io.WriteString(out, s)
	default:
		l = l - 65821
		out.Write([]byte{0x5f, byte((l & 0xFFFF00) >> 16), byte((l & 0xFF00) >> 8), byte(l & 0xFF)})
		io.WriteString(out, s)
	}
}

func write_uint16(out io.Writer, x uint16) {
	if x < 256 {
		out.Write([]byte{0xa1, byte(x & 0xFF)})
	} else {
		out.Write([]byte{0xa2, byte((x & 0xFF00) >> 8), byte(x & 0xFF)})
	}
}

func write_uint32(out io.Writer, x uint32) {
	switch {
	case x <= 0xFF:
		out.Write([]byte{0xc1, byte(x & 0xFF)})
	case x <= 0xFFFF:
		out.Write([]byte{0xc2, byte((x & 0xFF00) >> 8), byte(x & 0xFF)})
	case x <= 0xFFFFFF:
		out.Write([]byte{0xc3, byte((x & 0xFF0000) >> 16), byte((x & 0xFF00) >> 8), byte(x & 0xFF)})
	default:
		out.Write([]byte{0xc4, byte((x & 0xFF000000) >> 24), byte((x & 0xFF0000) >> 16), byte((x & 0xFF00) >> 8), byte(x & 0xFF)})
	}
}

func write_uint64(out io.Writer, x uint64) {
	switch {
	case x <= 0xFF:
		out.Write([]byte{0x01, 0x02, byte(x & 0xFF)})
	case x <= 0xFFFF:
		out.Write([]byte{0x02, 0x02, byte((x & 0xFF00) >> 8), byte(x & 0xFF)})
	case x <= 0xFFFFFF:
		out.Write([]byte{0x03, 0x02, byte((x & 0xFF0000) >> 16), byte((x & 0xFF00) >> 8), byte(x & 0xFF)})
	case x <= 0xFFFFFFFF:
		out.Write([]byte{0x04, 0x02, byte((x & 0xFF000000) >> 24), byte((x & 0xFF0000) >> 16), byte((x & 0xFF00) >> 8), byte(x & 0xFF)})
	case x <= 0xFFFFFFFFFF:
		out.Write([]byte{0x05, 0x02, byte((x & 0xFF00000000) >> 32), byte((x & 0xFF000000) >> 24), byte((x & 0xFF0000) >> 16), byte((x & 0xFF00) >> 8), byte(x & 0xFF)})
	case x <= 0xFFFFFFFFFFFF:
		out.Write([]byte{0x06, 0x02, byte((x & 0xFF0000000000) >> 40), byte((x & 0xFF00000000) >> 32), byte((x & 0xFF000000) >> 24), byte((x & 0xFF0000) >> 16), byte((x & 0xFF00) >> 8), byte(x & 0xFF)})
	case x <= 0xFFFFFFFFFFFFFF:
		out.Write([]byte{0x07, 0x02, byte((x & 0xFF000000000000) >> 48), byte((x & 0xFF0000000000) >> 40), byte((x & 0xFF00000000) >> 32), byte((x & 0xFF000000) >> 24), byte((x & 0xFF0000) >> 16), byte((x & 0xFF00) >> 8), byte(x & 0xFF)})
	default:
		out.Write([]byte{0x08, 0x02, byte((x & 0xFF00000000000000) >> 56), byte((x & 0xFF000000000000) >> 48), byte((x & 0xFF0000000000) >> 40), byte((x & 0xFF00000000) >> 32), byte((x & 0xFF000000) >> 24), byte((x & 0xFF0000) >> 16), byte((x & 0xFF00) >> 8), byte(x & 0xFF)})
	}
}

func write_float32(out io.Writer, x float32) {
	y := math.Float32bits(x)
	out.Write([]byte{0x04, 0x08, byte((y & 0xFF000000) >> 24), byte((y & 0xFF0000) >> 16), byte((y & 0xFF00) >> 8), byte(y & 0xFF)})
}

func write_map(out io.Writer, sz int) {
	if sz >= 29 {
		log.Fatal("maps >= 29 keys are not supported")
	}
	out.Write([]byte{0xe0 | byte(sz&0xFF)})
}

func write_array(out io.Writer, sz int) {
	if sz >= 29 {
		log.Fatal("arrays >= 29 are not supported")
	}
	out.Write([]byte{byte(sz & 0xFF), 0x04})
}

func write_ptr(out io.Writer, offset int) {
	switch {
	case offset < 2048:
		out.Write([]byte{byte(0x20 | ((offset >> 8) & 0x7)), byte(offset & 0xFF)})
	case offset < 526336:
		offset = offset - 2048
		out.Write([]byte{byte(0x28 | ((offset >> 16) & 0x7)), byte((offset >> 8) & 0xFF), byte(offset & 0xFF)})
	case offset < 134744064:
		offset = offset - 526336
		out.Write([]byte{byte(0x30 | ((offset >> 24) & 0x7)), byte((offset >> 16) & 0xFF), byte((offset >> 8) & 0xFF), byte(offset & 0xFF)})
	default:
		out.Write([]byte{0x38, byte((offset >> 24) & 0xFF), byte((offset >> 16) & 0xFF), byte((offset >> 8) & 0xFF), byte(offset & 0xFF)})
	}
}

func intern_string(s string) int {
	offset, ok := terms[s]
	if !ok {
		offset = data.Len()
		terms[s] = offset
		write_utf8string(data, s)
	}
	return offset
}

func intern_country(iso string, name string) int {
	offset, ok := terms["country\x00"+iso]
	if !ok {
		//country_key_offset := intern_string("country")
		names_key_offset := intern_string("names")
		iso_key_offset := intern_string("iso_code")
		//iso_offset := intern_string(iso)
		name_offset := intern_string(name)
		offset = data.Len()
		terms["country\x00"+iso] = offset
		write_map(data, 2)
		write_ptr(data, iso_key_offset)
		write_utf8string(data, iso)
		write_ptr(data, names_key_offset)
		write_map(data, 1)
		write_utf8string(data, "en")
		write_ptr(data, name_offset)
	}
	return offset
}

func intern_region(name string) int {
	offset, ok := terms["subdivisions\x00"+name]
	if !ok {
		names_key_offset := intern_string("names")
		name_offset := intern_string(name)
		offset = data.Len()
		terms["subdivisions\x00"+name] = offset
		write_array(data, 1)
		write_map(data, 1)
		write_ptr(data, names_key_offset)
		write_map(data, 1)
		write_utf8string(data, "en")
		write_ptr(data, name_offset)
	}
	return offset
}

func intern_city(name string) int {
	offset, ok := terms["city\x00"+name]
	if !ok {
		names_key_offset := intern_string("names")
		name_offset := intern_string(name)
		offset = data.Len()
		terms["city\x00"+name] = offset
		write_map(data, 1)
		write_ptr(data, names_key_offset)
		write_map(data, 1)
		write_utf8string(data, "en")
		write_ptr(data, name_offset)
	}
	return offset
}

func intern_geo(geo GeoName) int {
	hash := fnv.New64()
	fmt.Fprintln(hash, geo)
	hash64 := hash.Sum64()
	key := fmt.Sprintf("geo\x00%X", hash64)
	offset, ok := terms[key]
	if !ok {
		country_key_offset := intern_string("country")
		country_offset := intern_country(geo.Country_ISO, geo.Country)
		region_key_offset := intern_string("subdivisions")
		region_offset := intern_region(geo.Region)
		city_key_offset := intern_string("city")
		city_offset := intern_city(geo.City)
		loc_key_offset := intern_string("location")
		lat_key_offset := intern_string("latitude")
		long_key_offset := intern_string("longitude")
		no_lat_long := geo.Latitude == float32(math.NaN()) || geo.Longitude == float32(math.NaN())
		no_lat_long = false
		offset = data.Len()
		terms[key] = offset
		if no_lat_long {
			write_map(data, 3)
		} else {
			write_map(data, 4)
		}
		write_ptr(data, country_key_offset)
		write_ptr(data, country_offset)
		write_ptr(data, region_key_offset)
		write_ptr(data, region_offset)
		write_ptr(data, city_key_offset)
		write_ptr(data, city_offset)
		if !no_lat_long {
			write_ptr(data, loc_key_offset)
			write_map(data, 2)
			write_ptr(data, lat_key_offset)
			write_float32(data, geo.Latitude)
			write_ptr(data, long_key_offset)
			write_float32(data, geo.Longitude)
		}
	}
	return offset
}

// translate node to MaxMindDB convention
// > 0:  node index (1-based)
// == 0: nil
// < 0: geonode id (1-based) * -1
func translate(x int) uint32 {
	switch {
	case x > 0:
		return uint32(x - 1)
	case x == 0:
		return node_seq
	default:
		// invert the substraction of 1 done to avoid collision with x == 0
		// if -(x + 1) > data.Len() {
		// 	log.Fatal("excessive offset", x)
		// }
		return uint32(int(node_seq) - (x + 1) + 16)
	}
}

func record(a uint32, b uint32, record_size int) []byte {
	var out []byte

	switch record_size {
	case 24:
		out = make([]byte, 6)
		out[0] = byte((a & 0xFF0000) >> 16)
		out[1] = byte((a & 0xFF00) >> 8)
		out[2] = byte(a & 0xFF)
		out[3] = byte((b & 0xFF0000) >> 16)
		out[4] = byte((b & 0xFF00) >> 8)
		out[5] = byte(b & 0xFF)
	case 28:
		out = make([]byte, 7)
		out[0] = byte((a & 0xFF0000) >> 16)
		out[1] = byte((a & 0xFF00) >> 8)
		out[2] = byte(a & 0xFF)
		out[3] = byte(((a & 0xF000000) >> 20) | ((b & 0xF000000) >> 24))
		out[4] = byte((b & 0xFF0000) >> 16)
		out[5] = byte((b & 0xFF00) >> 8)
		out[6] = byte(b & 0xFF)
	case 32:
		out = make([]byte, 8)
		out[0] = byte((a & 0xFF000000) >> 24)
		out[1] = byte((a & 0xFF0000) >> 16)
		out[2] = byte((a & 0xFF00) >> 8)
		out[3] = byte(a & 0xFF)
		out[4] = byte((b & 0xFF000000) >> 24)
		out[5] = byte((b & 0xFF0000) >> 16)
		out[6] = byte((b & 0xFF00) >> 8)
		out[7] = byte(b & 0xFF)

	default:
		log.Fatal("record size", record_size, "not implemented")
	}
	return out
}

func Dump(fn string, record_size int) {
	if (1 << uint(record_size)) < node_seq {
		log.Fatal("record size ", record_size, " insufficient for ", node_seq)
	}
	if len(nodes) != int(node_seq) {
		log.Fatal("mismatch: len(nodes) = ", len(nodes), " node_seq = ", node_seq)
	}
	f, err := os.OpenFile(fn, os.O_CREATE|os.O_RDWR, 0644)
	defer f.Close()
	if err != nil {
		log.Fatal("could not open maxminddb file: ", err)
	}
	out := bufio.NewWriter(f)
	// binary tree
	for i := uint32(0); i < node_seq; i++ {
		a := translate(nodes[i].Bit0)
		b := translate(nodes[i].Bit1)
		rec := record(a, b, record_size)
		out.Write(rec)
	}
	// data section separator
	out.Write([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	// data section
	out.Write(data.Bytes())
	// metadata section
	out.WriteString("\xAB\xCD\xEFMaxMind.com")
	write_map(out, 9)
	write_utf8string(out, "binary_format_major_version")
	write_uint16(out, 2)
	write_utf8string(out, "binary_format_minor_version")
	write_uint16(out, 2)
	write_utf8string(out, "build_epoch")
	write_uint64(out, uint64(time.Now().Unix()))
	write_utf8string(out, "database_type")
	write_utf8string(out, "GeoIP2-City")
	write_utf8string(out, "description")
	write_map(out, 1)
	write_utf8string(out, "en")
	write_utf8string(out, "GeoIP2 City database")
	write_utf8string(out, "ip_version")
	write_uint16(out, 4)
	write_utf8string(out, "languages")
	write_array(out, 1)
	write_utf8string(out, "en")
	write_utf8string(out, "node_count")
	write_uint32(out, node_seq)
	write_utf8string(out, "record_size")
	write_uint16(out, uint16(record_size))

	out.Flush()
}
