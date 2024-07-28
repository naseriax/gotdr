/*
Author: Naseredin Aramnejad naseredin.aramnejad@gmail.com
This script is designed to extract all the possible information from the
given sor file.
each sor file (Provided by OTDR Equipment) contains multiple data blocks
and since it's a binary file, it should be red per byte.

Formulas and blueprint of this script are inspired by the information provided by:
Sidney Li
http://morethanfootnotes.blogspot.com/2015/07/
*/

package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

// lightSpeed is the speed of light in a vaccuum to be used for refractive index and fiber length calculation.
const lightSpeed = 299.79181901 // m/µsec

// errDealer handles the errors.
func errDealer(err error) {
	if err != nil {
		panic(err)
	}
}

func mod(a, b float64) float64 {
	return a - b*math.Floor(a/b)
}

func removePaths(stack []byte) []byte {
	lines := bytes.Split(stack, []byte("\n"))
	for i, line := range lines {
		if idx := bytes.LastIndex(line, []byte("/go/")); idx != -1 {
			lines[i] = line[idx+4:]
		} else if idx := bytes.Index(line, []byte(":")); idx != -1 {
			lines[i] = line[idx:]
		}
	}
	return bytes.Join(lines, []byte("\n"))
}

func customPanicHandler() {
	if r := recover(); r != nil {
		// Get the stack trace
		stack := debug.Stack()

		// Remove file paths from the stack trace
		sanitizedStack := removePaths(stack)

		// Log or print the sanitized stack trace
		fmt.Printf("Panic: %v\n%s", r, sanitizedStack)

		// Optionally, exit the program
		os.Exit(1)
	}
}

/*
ReadSorFile function opens the sor file and returns a hex string (hexData)
and a text string (charString) from the file to the main function,
Basically reading the whole file and putting it in RAM.
*/
func ReadSorFile(filename string) otdrRawData {

	r := otdrRawData{
		Filename: filename,
	}

	// var array []byte
	// var hexData string

	// f, err := os.Open(filename)
	// errDealer(err)

	// defer f.Close()

	// for {
	// 	eachByte := make([]byte, 1)
	// 	_, err = f.Read(eachByte)

	// 	if err == io.EOF {
	// 		break
	// 	}
	// 	errDealer(err)

	// 	array = append(array, eachByte...)
	// }
	f, err := os.Open(filename)
	if err != nil {
		errDealer(err)
	}
	defer f.Close()

	// Get the file size
	stat, err := f.Stat()
	if err != nil {
		errDealer(err)
	}

	// Read the entire file at once
	buffer := make([]byte, stat.Size())
	_, err = io.ReadFull(f, buffer)
	if err != nil {
		errDealer(err)
	}

	//Converting the byte array into a hex String
	r.HexData = hex.EncodeToString(buffer)

	//Converting the HexData to a text string
	chars, err := hex.DecodeString(r.HexData)
	errDealer(err)

	r.Decodedfile = string(chars)
	return r
}

// Reverse will reverse the hex string in every 2 bytes. Example: 0ABCD123 => 23D1BC0A.
func Reverse(s string) string {
	str := ""
	for ind := 0; ind < len(s); ind += 2 {
		str = s[ind:ind+2] + str
	}
	return str
}

func dB(point int64) float64 {
	return float64(point*-1000) * math.Pow(10, -6)
}

// parsHexValue calls the Reverse() funcition to reverse the order of the provided HexString and then converts it's value to int64.
func parsHexValue(hexData string) int64 {
	output, err := strconv.ParseInt(Reverse(hexData), 16, 64)
	if err != nil {
		fmt.Println(err)
		return 0
	}
	return output
}

func (d *otdrRawData) mapKeyEvents(events string) map[int][2]int {
	m := make(map[int][2]int)
	start := 4
	evnumbers := int(parsHexValue(events[:start]))

	for i := 1; i <= evnumbers; i++ {
		if i == evnumbers {
			m[i] = [2]int{start, len(events) - 46}
		} else {
			end := strings.Index(events[start+84:], fmt.Sprintf("%02x00", i+1))
			if end == -1 {
				fmt.Println("pattern not found:", events)
				return nil
			}
			end += start + 84
			m[i] = [2]int{start, end}
			start = end
		}
	}

	return m
}

func (d *otdrRawData) GetNext(key string) string {
	if _, exists := d.SecLocs[key]; !exists {
		return ""
	}

	index := d.SecLocs[key][0]
	var nextKey string
	nextIndex := math.Inf(1)

	for k, v := range d.SecLocs {
		if k == key || len(v) == 0 {
			continue
		}
		if float64(index) < float64(v[0]) && float64(v[0]) < nextIndex {
			nextIndex = float64(v[0])
			nextKey = k
		}
	}

	return nextKey
}

func (d *otdrRawData) GetOrder() {
	sections := []string{
		"SupParams",
		"ExfoNewProprietaryBlock",
		"Map",
		"FxdParams",
		"DataPts",
		"NokiaParams",
		"KeyEvents",
		"GenParams",
		"WaveMTSParams",
		"WavetekTwoMTS",
		"WavetekThreeMTS",
		"BlocOtdrPrivate",
		"ActernaConfig",
		"ActernaMiniCurve",
		"JDSUEvenementsMTS",
		"Cksum",
	}

	sectionLocations := make(map[string][]int)

	for _, word := range sections {
		re := regexp.MustCompile(regexp.QuoteMeta(word))
		matches := re.FindAllStringIndex(d.Decodedfile, -1)
		locations := make([]int, len(matches))
		for i, match := range matches {
			locations[i] = match[0]
		}
		sectionLocations[word] = locations
	}

	if len(sectionLocations["Cksum"]) < 2 {
		fmt.Printf("%s file has no checksum\n", d.Filename)
		os.Exit(1)
	}

	d.SecLocs = sectionLocations
}

// FixedParams function extracts the Fixed Parameters from the sor file and stores it in FixInfos struct.
func (d *otdrRawData) getFixedParams() {

	f := FixInfo{}

	fixInfo := d.HexData[(d.SecLocs["FxdParams"][1]+10)*2 : (d.SecLocs[d.GetNext("FxdParams")][1] * 2)]
	p := 8

	f.DateTime = time.Unix(parsHexValue(fixInfo[:p]), 0)

	unit, err := hex.DecodeString(fixInfo[p : p+4])
	errDealer(err)
	p += 4

	f.ActualWL = float64(parsHexValue(fixInfo[p:p+4])) / 10.0
	p += 4

	f.AO = float64(parsHexValue(fixInfo[p : p+8]))
	p += 8

	f.AOD = float64(parsHexValue(fixInfo[p : p+8]))
	p += 8

	f.PulseWidthNo = parsHexValue(fixInfo[p : p+4])
	p += 4

	for i := 0; i < int(f.PulseWidthNo); i++ {
		f.PulseWidth = append(f.PulseWidth, parsHexValue(fixInfo[p:p+4]))
		p += 4
	}

	resolution_m_p1 := []float64{}
	for i := 0; i < int(f.PulseWidthNo); i++ {
		resolution_m_p1 = append(resolution_m_p1, float64(parsHexValue(fixInfo[p:p+8]))*math.Pow(10, -8))
		p += 8
	}

	for i := 0; i < int(f.PulseWidthNo); i++ {
		f.SampleQTY = append(f.SampleQTY, parsHexValue(fixInfo[p:p+8]))
		p += 8
	}

	f.IOR = float64(parsHexValue(fixInfo[p : p+8]))
	p += 8

	f.RefIndex = f.IOR * math.Pow(10, -5)

	f.FiberSpeed = lightSpeed / f.RefIndex

	for i := 0; i < int(f.PulseWidthNo); i++ {
		f.Resolution = append(f.Resolution, resolution_m_p1[i]*f.FiberSpeed)
	}

	f.Backscattering = float64(parsHexValue(fixInfo[p:p+4])) * -0.1
	p += 4

	f.Averaging = parsHexValue(fixInfo[p : p+8])
	p += 8

	f.AveragingTime = float64(parsHexValue(fixInfo[p:p+4])) / 600
	p += 4

	for i := 0; i < int(f.PulseWidthNo); i++ {
		f.Range = append(f.Range, float64(f.SampleQTY[i])*f.Resolution[i])
	}

	f.Unit = string(unit)

	d.FixedParams = f

}

func (d *otdrRawData) getDataPoints() {
	dtpoints := d.HexData[d.SecLocs["DataPts"][1]*2 : d.SecLocs[d.GetNext("DataPts")][1]*2][40:]
	start := 0
	cumulative_length := 0

	for i := range d.FixedParams.SampleQTY {

		qty := int(d.FixedParams.SampleQTY[i])
		resolution := int(d.FixedParams.Resolution[i])

		for j := 0; j < qty; j++ {
			index := start + j
			if index < len(dtpoints) {
				hex_value := dtpoints[index*4 : index*4+4]
				db_value := dB(parsHexValue(hex_value))
				passedlen := math.Round(float64(cumulative_length*1000)) / 1000
				dataPoint := []float64{passedlen, db_value}
				d.Distance = append(d.Distance, passedlen)
				d.Power = append(d.Power, db_value)
				d.DataPoints = append(d.DataPoints, dataPoint)
				cumulative_length += resolution
			}
		}
		start += qty
	}

}

// SupParams function extracts the Supplier Parameters from the sor file and stores it in SupParam struct.
func (d *otdrRawData) getSupParams() {
	supString := strings.Split(d.Decodedfile[d.SecLocs["SupParams"][1]+10:d.SecLocs[d.GetNext("SupParams")][1]], "\x00")
	slicedParams := supString[:len(supString)-1]

	supInfo := SupParam{
		OTDRSupplier:   strings.TrimSpace(slicedParams[0]),
		OTDRName:       strings.TrimSpace(slicedParams[1]),
		OTDRsn:         strings.TrimSpace(slicedParams[2]),
		OTDRModuleName: strings.TrimSpace(slicedParams[3]),
		OTDRModuleSN:   strings.TrimSpace(slicedParams[4]),
		OTDRswVersion:  strings.TrimSpace(slicedParams[5]),
		OTDROtherInfo:  strings.TrimSpace(slicedParams[6]),
	}

	d.Supplier = supInfo
}

// GenParams function extracts the General Parameters from the sor file and stores it in GenParam struct.
func (d *otdrRawData) getGenParams() {

	genStringBeforeSplit := strings.Split(d.Decodedfile[d.SecLocs["GenParams"][1]+10:d.SecLocs[d.GetNext("GenParams")][1]], "\x00")
	genString := genStringBeforeSplit[:len(genStringBeforeSplit)-1]

	genInfo := GenParam{
		CableID:        strings.TrimSpace(genString[0][2:]),
		Lang:           strings.TrimSpace(genString[0][:2]),
		FiberID:        strings.TrimSpace(genString[1]),
		LocationA:      strings.TrimSpace(genString[2][4:]),
		LocationB:      strings.TrimSpace(genString[3]),
		CableCode:      strings.TrimSpace(genString[4]),
		BuildCondition: genString[5],
		Operator:       strings.TrimSpace(genString[13]),
		Comment:        strings.TrimSpace(genString[14]),
		FiberType:      "G." + strconv.FormatInt(parsHexValue(hex.EncodeToString([]byte(genString[2][:2]))), 10),
		OTDRWavelength: strconv.FormatInt(parsHexValue(hex.EncodeToString([]byte(genString[2][2:4]))), 10) + " nm",
	}

	d.GenParams = genInfo
}

// FiberLength calculates the fiber length and returns it.
func (d *otdrRawData) getFiberLength() {
	for _, v := range d.Events {
		if strings.Contains(v.EventType, "EXX") || strings.Contains(v.EventType, "E99") {
			d.TotalLength = float64(v.EventLocM)
		}
	}
}

// BellCoreVersion reads the bellcore version from the sor file and returns it.
func (d *otdrRawData) getBellCoreVersion() {
	d.BellCoreVersion = float32(parsHexValue(d.HexData[(d.SecLocs["Map"][0]+4)*2:(d.SecLocs["Map"][0]+5)*2])) / 100.0
}

// TotalLoss reads the total loss of the fiber from the sor file and returns it.
func (d *otdrRawData) getTotalLoss() {
	if len(d.SecLocs["WaveMTSParams"]) > 0 {
		totallossinfo := d.HexData[(d.SecLocs["WaveMTSParams"][1]-22)*2 : (d.SecLocs["WaveMTSParams"][1]-18)*2]
		d.TotalLoss = float32(parsHexValue(totallossinfo)) * 0.001
	} else {
		d.TotalLoss = 0
	}
}

// KeyEvents function extracts the events information from the sor file and stores it in OTDREvent struct.
func (d *otdrRawData) getKeyEvents() {

	d.Events = map[int]OTDREvent{}

	events := d.HexData[(d.SecLocs["KeyEvents"][1]+10)*2 : d.SecLocs[d.GetNext("KeyEvents")][1]*2]
	p := d.mapKeyEvents(events)

	var eventhexlist []string
	for _, v := range p {
		eventhexlist = append(eventhexlist, events[v[0]:v[1]])
	}

	for _, e := range eventhexlist {

		event := OTDREvent{}
		eNum := int(parsHexValue(e[:4]))

		event.EventLocM = float64(parsHexValue(e[4:12])) * (math.Pow(10, -4)) * float64(d.FixedParams.FiberSpeed)

		stValue := mod(event.EventLocM, d.FixedParams.Resolution[0])
		if stValue >= d.FixedParams.Resolution[0]/2 {
			event.EventLocM = event.EventLocM + (d.FixedParams.Resolution[0] - stValue)
		} else {
			event.EventLocM = event.EventLocM + -stValue
		}

		event.Slope = float32(parsHexValue(e[12:16])) * 0.001
		event.SpliceLoss = float32(parsHexValue(e[16:20])) * 0.001
		if parsHexValue(e[20:28]) > 0 {
			event.RefLoss = float32((float64(parsHexValue(e[20:28])) - math.Pow(2, 32)) * 0.001)
		} else {
			event.RefLoss = float32(parsHexValue(e[20:28]))
		}

		eventType, err := hex.DecodeString(e[28:44])
		errDealer(err)
		event.EventType = string(eventType)
		event.EndOfPreviousEvent = int(parsHexValue(e[44:52]))
		event.BegOfCurrentEvent = int(parsHexValue(e[52:60]))
		event.EndOfCurrentEvent = int(parsHexValue(e[60:68]))
		event.BegOfNextEvent = int(parsHexValue(e[68:76]))
		event.PeakCurrentEvent = int(parsHexValue(e[76:84]))
		if len(e) > 88 {
			if len(e) < 102 {
				comment, err := hex.DecodeString(e[84:])
				errDealer(err)
				event.Comment = string(comment)
			} else {
				comment, err := hex.DecodeString(e[84:102])
				errDealer(err)
				event.Comment = string(comment)
			}
		}
		d.Events[eNum] = event
	}
}

func (d *otdrRawData) export2Json() {
	b, err := json.MarshalIndent(d, "", "  ")
	errDealer(err)
	_ = os.WriteFile("OTDR_Output.json", b, 0644)
	fmt.Println("Json file has been exported! - json file name: OTDR_Output.json")
}

func getCliArgs() string {

	filePath := flag.String("sorfile", "", "Path to the sor file")
	flag.StringVar(filePath, "s", "", "Path to the sor file")

	flag.Parse()

	return *filePath
}

func main() {

	defer customPanicHandler()

	d := ReadSorFile(getCliArgs())
	d.GetOrder()
	d.getBellCoreVersion()
	d.getTotalLoss()
	d.getSupParams()
	d.getGenParams()
	d.getFixedParams()
	d.getDataPoints()
	d.getKeyEvents()
	d.getFiberLength()
	d.export2Json()
}
