package main

import (
	"bufio"
	"encoding/csv"
	"flag"
	"io"
	"log"
	"math"
	"net"
	"os"
	"runtime/pprof"
	"strconv"

	"github.com/singular-labs/maxminddb"
)

type DataSource struct {
	f     *os.File
	buf   *bufio.Reader
	csv   *csv.Reader
	begin uint32
	end   uint32
}

var (
	verbose  *bool
	geonames map[int]maxminddb.GeoName
)

func read_geonames(gfn string) map[int]maxminddb.GeoName {
	if *verbose {
		log.Println("reading geonames")
	}
	geonames := make(map[int]maxminddb.GeoName, 10000)
	f, err := os.Open(gfn)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	_, err = r.Read() // read header
	if err != nil {
		log.Fatal(err)
	}
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		id, err := strconv.Atoi(record[0])
		if err != nil {
			log.Fatal(err)
		}
		// if *verbose {
		// 	log.Println("record", id, record)
		// }
		geonames[id] = maxminddb.GeoName{record[4], record[5], record[7], record[10], float32(math.NaN()), float32(math.NaN())}
	}
	geonames[-1] = maxminddb.GeoName{"--", "Unknown", "Unknown", "Unknown", float32(math.NaN()), float32(math.NaN())}
	return geonames
}

func open_data_source(fn string) DataSource {
	var err error

	out := DataSource{}
	out.f, err = os.Open(fn)
	if err != nil {
		log.Fatal(err)
	}
	out.buf = bufio.NewReader(out.f)
	out.csv = csv.NewReader(out.buf)
	return out
}

func (d *DataSource) next() (begin net.IP, end net.IP, next *maxminddb.GeoName) {
	record, err := d.csv.Read()
	if err == io.EOF {
		return
	}
	if err != nil {
		log.Fatal(err)
	}
	// if *verbose {
	// 	log.Println("AAAAAAAA", record)
	// }
	_, block, err := net.ParseCIDR(record[0])
	if err != nil {
		log.Fatal(err)
	}
	geoname_str := record[1]
	if geoname_str == "" {
		geoname_str = record[2]
	}
	geoname_id, err := strconv.Atoi(geoname_str)
	if err != nil {
		if record[1] != "" {
			log.Println("warning: could not read geoname_id for", record, err)
		}
		geoname_id = -1
	}
	geoname, ok := geonames[geoname_id]
	if !ok {
		log.Fatal("unknown geoname", geoname_id)
	}
	ones, bits := block.Mask.Size()
	if bits != 32 {
		log.Fatal("not an IPv4 block", block)
	}
	blocksize := uint32(1 << uint(bits-ones))
	begin = block.IP
	end = maxminddb.Uint32_to_IP(maxminddb.IP_to_uint32(block.IP) + blocksize - 1)
	copy := new(maxminddb.GeoName)
	*copy = geoname
	lat, err := strconv.ParseFloat(record[7], 32)
	if err != nil {
		lat = math.NaN()
	}
	long, err := strconv.ParseFloat(record[8], 32)
	if err != nil {
		long = math.NaN()
	}
	copy.Latitude = float32(lat)
	copy.Longitude = float32(long)
	next = copy
	return
}

func main() {
	// command-line options
	verbose = flag.Bool("v", false, "Verbose error reporting")
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to file")
	mfn := flag.String("mmdb", "Singular.mmdb", "filename for MMDB file")
	cfn := flag.String("csv", "", "filename for IP blocks CSV file")
	gfn := flag.String("geo", "", "filename for GeoNames CSV file")
	flag.Parse()

	var err error

	if *cfn == "" {
		log.Fatal("must specify an IP blocks CSV file using -csv")
	}
	if *gfn == "" {
		log.Fatal("must specify a geonames CSV file using -geo")
	}

	// Profiler
	if *cpuprofile != "" {
		var f *os.File
		f, err = os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	geonames = read_geonames(*gfn)

	m := open_data_source(*cfn)
	m.csv.Read() // pop header line
	begin, end, next := m.next()
	for next != nil {
		maxminddb.Push_Range(begin, end, *next)
		begin, end, next = m.next()
	}
	if *mfn != "" {
		maxminddb.Dump(*mfn, 28)
	}
}
