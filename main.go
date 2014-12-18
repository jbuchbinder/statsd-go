package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/jbuchbinder/go-gmetric/gmetric"
	"log"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	TCP = "tcp"
	UDP = "udp"
)

type Packet struct {
	Bucket   string
	Value    string
	Modifier string
	Sampling float32
}

var (
	serviceAddress   = flag.String("address", ":8125", "UDP service address")
	graphiteAddress  = flag.String("graphite", "", "Graphite service address (example: 'localhost:2003')")
	gangliaAddress   = flag.String("ganglia", "", "Ganglia gmond servers, comma separated")
	gangliaPort      = flag.Int("ganglia-port", 8649, "Ganglia gmond service port")
	gangliaSpoofHost = flag.String("ganglia-spoof-host", "", "Ganglia gmond spoof host string")
	gangliaGroup     = flag.String("ganglia-group", "statsd", "Ganglia metric group name")
	flushInterval    = flag.Int64("flush-interval", 10, "Flush interval")
	percentThreshold = flag.Int("percent-threshold", 90, "Threshold percent")
	statsPrefix      = flag.String("stats-prefix", "stats.", "Counters Prefix")
	countersPrefix   = flag.String("counters-prefix", "stats.counters.", "Counters Prefix")
	gaugesPrefix     = flag.String("gauges-prefix", "stats.gauges.", "Gauges Prefix")
	timersPrefix     = flag.String("timers-prefix", "stats.timers.", "Timers Prefix")
	debug            = flag.Bool("debug", false, "Debug mode")
)

var (
	In       = make(chan Packet, 10000)
	counters = make(map[string]int)
	timers   = make(map[string][]float64)
	gauges   = make(map[string]int)
)

func monitor() {
	var err error
	if err != nil {
		log.Println(err)
	}
	t := time.NewTicker(time.Duration(*flushInterval) * time.Second)
	for {
		select {
		case <-t.C:
			submit()
		case s := <-In:
			if s.Modifier == "ms" {
				_, ok := timers[s.Bucket]
				if !ok {
					var t []float64
					timers[s.Bucket] = t
				}
				//intValue, _ := strconv.Atoi(s.Value)
				floatValue, _ := strconv.ParseFloat(s.Value, 64)
				timers[s.Bucket] = append(timers[s.Bucket], floatValue)
			} else if s.Modifier == "g" {
				_, ok := gauges[s.Bucket]
				if !ok {
					gauges[s.Bucket] = 0
				}
				if strings.HasPrefix(s.Value, "+") {
					intValue, _ := strconv.Atoi(s.Value[1:])
					gauges[s.Bucket] += intValue
				} else if strings.HasPrefix(s.Value, "-") {
					intValue, _ := strconv.Atoi(s.Value[1:])
					gauges[s.Bucket] -= intValue
				} else {
					intValue, _ := strconv.Atoi(s.Value)
					gauges[s.Bucket] = intValue
				}
			} else {
				_, ok := counters[s.Bucket]
				if !ok {
					counters[s.Bucket] = 0
				}
				floatValue, _ := strconv.ParseFloat(s.Value, 32)
				counters[s.Bucket] += int(float32(floatValue) * (1 / s.Sampling))
			}
		}
	}
}

func submit() {
	var clientGraphite net.Conn
	if *graphiteAddress != "" {
		var err error
		clientGraphite, err = net.Dial(TCP, *graphiteAddress)
		if clientGraphite != nil {
			// Run this when we're all done, only if clientGraphite was opened.
			defer clientGraphite.Close()
		}
		if err != nil {
			log.Printf(err.Error())
		}
	}
	var useGanglia bool
	var gm gmetric.Gmetric
	gmSubmit := func(name string, value uint32) {
		if useGanglia {
			if *debug {
				log.Printf("Ganglia send metric %s value %d", name, value)
			}
			m_value := fmt.Sprint(value)
			m_units := "count"
			m_type := uint32(gmetric.VALUE_UNSIGNED_INT)
			m_slope := uint32(gmetric.SLOPE_BOTH)
			m_grp := *gangliaGroup
			m_ival := uint32(*flushInterval * int64(2))

			go gm.SendMetric(name, m_value, m_type, m_units, m_slope, m_ival, m_ival, m_grp)
		}
	}
	gmSubmitFloat := func(name string, value float64) {
		if useGanglia {
			if *debug {
				log.Printf("Ganglia send float metric %s value %f", name, value)
			}
			m_value := fmt.Sprint(value)
			m_units := "count"
			m_type := uint32(gmetric.VALUE_DOUBLE)
			m_slope := uint32(gmetric.SLOPE_BOTH)
			m_grp := "statsd"
			m_ival := uint32(*flushInterval * int64(2))

			go gm.SendMetric(name, m_value, m_type, m_units, m_slope, m_ival, m_ival, m_grp)
		}
	}
	if *gangliaAddress != "" {
		gm = gmetric.Gmetric{
			Host:  *gangliaSpoofHost,
			Spoof: *gangliaSpoofHost,
		}
		gm.SetVerbose(false)

		if strings.Contains(*gangliaAddress, ",") {
			segs := strings.Split(*gangliaAddress, ",")
			for i := 0; i < len(segs); i++ {
				gIP, err := net.ResolveIPAddr("ip4", segs[i])
				if err != nil {
					panic(err.Error())
				}
				gm.AddServer(gmetric.Server{Server: gIP.IP, Port: *gangliaPort})
			}
		} else {
			gIP, err := net.ResolveIPAddr("ip4", *gangliaAddress)
			if err != nil {
				panic(err.Error())
			}
			gm.AddServer(gmetric.Server{Server: gIP.IP, Port: *gangliaPort})
		}
		useGanglia = true
	} else {
		useGanglia = false
	}
	numStats := 0
	now := int32(time.Now().Unix())
	buffer := bytes.NewBufferString("")
	for s, c := range counters {
		value := float64(c) / float64((float64(*flushInterval)*float64(time.Second))/float64(1e3))
		fmt.Fprintf(buffer, "%s%s %d %d\n", *statsPrefix, s, value, now)
		gmSubmitFloat(fmt.Sprintf("stats_%s", s), value)
		fmt.Fprintf(buffer, "%s%s %d %d\n", *countersPrefix, s, c, now)
		gmSubmit(fmt.Sprintf("stats_counts_%s", s), uint32(c))
		counters[s] = 0
		numStats++
	}
	for i, g := range gauges {
		value := int64(g)
		fmt.Fprintf(buffer, "%s%s %d %d\n", *gaugesPrefix, i, value, now)
		gmSubmit(fmt.Sprintf("stats_%s", i), uint32(value))
		numStats++
	}
	for u, t := range timers {
		if len(t) > 0 {
			sort.Float64s(t)
			min := float64(t[0])
			max := float64(t[len(t)-1])
			mean := float64(min)
			maxAtThreshold := float64(max)
			count := len(t)
			if len(t) > 1 {
				var thresholdIndex int
				thresholdIndex = ((100 - *percentThreshold) / 100) * count
				numInThreshold := count - thresholdIndex
				values := t[0:numInThreshold]

				sum := float64(0)
				for i := 0; i < numInThreshold; i++ {
					sum += values[i]
				}
				mean = float64(sum) / float64(numInThreshold)
			}
			var z []float64
			timers[u] = z

			fmt.Fprintf(buffer, "%s%s.mean %f %d\n", *timersPrefix, u, mean, now)
			gmSubmitFloat(fmt.Sprintf("stats_timers_%s_mean", u), mean)
			fmt.Fprintf(buffer, "%s%s.upper %f %d\n", *timersPrefix, u, max, now)
			gmSubmitFloat(fmt.Sprintf("stats_timers_%s_upper", u), max)
			fmt.Fprintf(buffer, "%s%s.upper_%d %f %d\n", *timersPrefix, u,
				*percentThreshold, maxAtThreshold, now)
			gmSubmitFloat(fmt.Sprintf("stats_timers_%s_upper_%d", u, *percentThreshold), maxAtThreshold)
			fmt.Fprintf(buffer, "%s%s.lower %f %d\n", *timersPrefix, u, min, now)
			gmSubmitFloat(fmt.Sprintf("stats_timers_%s_lower", u), min)
			fmt.Fprintf(buffer, "%s%s.count %d %d\n", *timersPrefix, u, count, now)
			gmSubmit(fmt.Sprintf("stats_timers_%s_count", u), uint32(count))
		} else {
			// Need to still submit timers as zero
			fmt.Fprintf(buffer, "%s%s.mean %f %d\n", *timersPrefix, u, 0.0, now)
			gmSubmitFloat(fmt.Sprintf("stats_timers_%s_mean", u), 0.0)
			fmt.Fprintf(buffer, "%s%s.upper %f %d\n", *timersPrefix, u, 0.0, now)
			gmSubmitFloat(fmt.Sprintf("stats_timers_%s_upper", u), 0.0)
			fmt.Fprintf(buffer, "%s%s.upper_%d %f %d\n", *timersPrefix, u,
				*percentThreshold, 0.0, now)
			gmSubmitFloat(fmt.Sprintf("stats_timers_%s_upper_%d", u, *percentThreshold), 0.0)
			fmt.Fprintf(buffer, "%s%s.lower %f %d\n", *timersPrefix, u, 0.0, now)
			gmSubmitFloat(fmt.Sprintf("stats_timers_%s_lower", u), 0.0)
			fmt.Fprintf(buffer, "%s%s.count %d %d\n", *timersPrefix, u, 0, now)
			gmSubmit(fmt.Sprintf("stats_timers_%s_count", u), uint32(0))
		}
		numStats++
	}
	fmt.Fprintf(buffer, "%sstatsd.numStats %d %d\n", *statsPrefix, numStats, now)
	gmSubmit("statsd_numStats", uint32(numStats))
	if clientGraphite != nil {
		if *debug {
			log.Println(fmt.Sprintf("Send to graphite: [[[%s]]]\n", string(buffer.Bytes())))
		}
		clientGraphite.Write(buffer.Bytes())
	}
}

func handleMessage(conn *net.UDPConn, remaddr net.Addr, buf *bytes.Buffer) {
	var packet Packet
	var value string
	var sanitizeRegexp = regexp.MustCompile("[^a-zA-Z0-9\\-_\\.:\\|@]")
	var packetRegexp = regexp.MustCompile("([a-zA-Z0-9_\\.]+):(\\-?[0-9\\.]+)\\|(c|ms|g)(\\|@([0-9\\.]+))?")
	s := sanitizeRegexp.ReplaceAllString(buf.String(), "")
	for _, item := range packetRegexp.FindAllStringSubmatch(s, -1) {
		value = item[2]
		if item[3] == "ms" {
			_, err := strconv.ParseFloat(item[2], 32)
			if err != nil {
				value = "0"
			}
		}

		sampleRate, err := strconv.ParseFloat(item[5], 32)
		if err != nil {
			sampleRate = 1
		}

		packet.Bucket = item[1]
		packet.Value = value
		packet.Modifier = item[3]
		packet.Sampling = float32(sampleRate)

		if *debug {
			log.Println(
				fmt.Sprintf("Packet: bucket = %s, value = %s, modifier = %s, sampling = %f\n",
					packet.Bucket, packet.Value, packet.Modifier, packet.Sampling))
		}

		In <- packet
	}
}

func udpListener() {
	address, _ := net.ResolveUDPAddr(UDP, *serviceAddress)
	listener, err := net.ListenUDP(UDP, address)
	defer listener.Close()
	if err != nil {
		log.Fatalf("ListenAndServe: %s", err.Error())
	}
	for {
		message := make([]byte, 512)
		n, remaddr, error := listener.ReadFrom(message)
		if error != nil {
			continue
		}
		buf := bytes.NewBuffer(message[0:n])
		if *debug {
			log.Println("Packet received: " + string(message[0:n]) + "\n")
		}
		go handleMessage(listener, remaddr, buf)
	}
}

func main() {
	flag.Parse()
	go udpListener()
	monitor()
}
