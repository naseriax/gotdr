/*
Author: Naseredin Aramnejad naseredin.aramnejad@gmail.com
This script is designed to extract all the possible information from the
given sor file.
each sor file (Provided by OTDR Equipment) contains multiple data blocks.
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
	"log"
	"math"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
)

const lightSpeed = 299.79181901 // m/µsec

func nukeIfErr(err error) {
	if err != nil {
		log.Fatalln(err.Error())
	}
}

func openBrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}

	if err != nil {
		fmt.Printf("Failed to open browser: %v\n", err)
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

func ReadSorFile(filename string) otdrRawData {

	r := otdrRawData{
		Filename: filename,
	}

	f, err := os.Open(filename)
	if err != nil {
		nukeIfErr(err)
	}
	defer f.Close()

	// Get the file size
	stat, err := f.Stat()
	if err != nil {
		nukeIfErr(err)
	}

	// Read the entire file at once
	buffer := make([]byte, stat.Size())
	_, err = io.ReadFull(f, buffer)
	if err != nil {
		nukeIfErr(err)
	}

	//Converting the byte array into a hex String
	r.HexData = hex.EncodeToString(buffer)

	//Converting the HexData to a text string
	chars, err := hex.DecodeString(r.HexData)
	nukeIfErr(err)

	r.Decodedfile = string(chars)
	return r
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

func (d *otdrRawData) draw() {

	// Create a new line chart instance
	xValues := make([]opts.LineData, len(d.DataPoints))
	yValues := make([]opts.LineData, len(d.DataPoints))
	// yValues := make([]opts.ScatterData, len(d.DataPoints))

	for i, point := range d.DataPoints {
		xValues[i] = opts.LineData{Value: point[0]}
		yValues[i] = opts.LineData{Value: point[1]}
		// yValues[i] = opts.ScatterData{Value: point[1]}
	}

	// Create a new line chart instance
	line := charts.NewLine()
	// line := charts.NewScatter()

	// Set global options like title and legend
	line.SetGlobalOptions(
		charts.WithColorsOpts(opts.Colors{
			"green",
		}),
		charts.WithToolboxOpts(opts.Toolbox{
			Show: opts.Bool(true),
			Feature: &opts.ToolBoxFeature{
				DataZoom: &opts.ToolBoxFeatureDataZoom{
					Show:       opts.Bool(true),
					YAxisIndex: false,
				},
				Restore: &opts.ToolBoxFeatureRestore{
					Show: opts.Bool(true),
				},
				SaveAsImage: &opts.ToolBoxFeatureSaveAsImage{
					Show: opts.Bool(true),
				},
			},
		}),

		charts.WithInitializationOpts(opts.Initialization{
			Width:  "1300px",
			Height: "500px",
		}),
		charts.WithDataZoomOpts(opts.DataZoom{
			Type:       "inside",
			Start:      0,
			End:        100,
			XAxisIndex: []int{0},
		}),
		charts.WithDataZoomOpts(opts.DataZoom{
			Type:       "slider",
			Start:      0,
			End:        100,
			XAxisIndex: []int{0},
		}),
		charts.WithLegendOpts(opts.Legend{Show: opts.Bool(true)}),
		charts.WithAnimation(true),

		charts.WithTitleOpts(opts.Title{
			Title: "GOTDR Viewer",
		}),
	)

	markPoints := make([]opts.MarkPointNameCoordItem, 0, len(d.DataPoints))

	for _, ev := range d.Events {
		loc := d.return_index(ev.EventLocM)

		if loc[0] == 0 {
			continue
		}

		c := "blue"
		if strings.Contains(ev.EventType, "EXXX") || strings.Contains(ev.EventType, "E999") {
			c = "red"
		}

		mkpoint := opts.MarkPointNameCoordItem{
			Name:       ev.EventType,
			Value:      strconv.Itoa(ev.EventNumber),
			Coordinate: []interface{}{loc[0], loc[1]},
			Symbol:     "pin",
			ItemStyle: &opts.ItemStyle{
				Color:   c,
				Opacity: 0.5,
			},
			SymbolSize: 35,
		}
		markPoints = append(markPoints, mkpoint)

	}

	// Add data to the line chart
	line.SetXAxis(xValues).AddSeries("Reflection", yValues, charts.WithMarkPointNameCoordItemOpts(markPoints...))
	line.SetSeriesOptions(
		charts.WithMarkPointStyleOpts(
			opts.MarkPointStyle{Label: &opts.Label{Show: opts.Bool(true)}}),
		charts.WithAreaStyleOpts(opts.AreaStyle{
			Color:   "yellow",
			Opacity: 0.1,
		}),
	)

	f, _ := os.Create("graph.html")
	// line.Render(f)
	d.generateHTML(f, line)

	openBrowser("graph.html")
}

func (d *otdrRawData) generateHTML(w io.Writer, line *charts.Line) {
	w.Write([]byte(`
    <!DOCTYPE html>
    <html>
    <head>
        <meta charset="utf-8">
        <title>KPI Report</title>
		<link rel="stylesheet" href="style.css">
    </head>
    <body>
        <div class="container">
            <div class="chart">
    `))

	line.Render(w)

	tmpl := template.Must(template.New("summary").Parse(`
 			</div>
            <div class="summary">
                <h2>Summary</h2>
                <table>
                    <thead>
                        <tr>
                            <th>Fiber Attribute</th>
                            <th>Value</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr>
                            <td>Date & Time</td>
                            <td>{{.DT}}</td>
                        </tr>
                        <tr>
                            <td>Unit</td>
                            <td>{{.UNIT}}</td>
                        </tr>
						<tr>
                            <td>Actual Wavelength</td>
                            <td>{{.WL}} nm</td>
                        </tr>
						<tr>
                            <td>Pulse Width QTY</td>
                            <td>{{.PWQ}}</td>
                        </tr>
						<tr>
                            <td>Pulse Width(ns)</td>
                            <td>{{.PW}}</td>
                        </tr>
						<tr>
                            <td>Sample Quantity</td>
                            <td>{{.SQ}}</td>
                        </tr>
						<tr>
                            <td>Fiber Light Speed</td>
                            <td>{{.FLS}} km/s</td>
                        </tr>
						<tr>
                            <td>Fiber Length (EOF)</td>
                            <td>{{.FLEN}} m</td>
                        </tr>
						<tr>
                            <td>Bellcore Version</td>
                            <td>{{.BLV}}</td>
                        </tr>
						<tr>
                            <td>Key Events</td>
                            <td>{{.KE}}</td>
                        </tr>
                    </tbody>
                </table>
            </div>
        </div>
    </body>
    </html>
`))

	data := struct {
		DT   time.Time
		UNIT string
		WL   float64
		PWQ  int64
		FLS  float64
		PW   []int64
		SQ   []int64
		FLEN float64
		BLV  float64
		KE   int
	}{
		DT:   d.FixedParams.DateTime,
		UNIT: d.FixedParams.Unit,
		WL:   d.FixedParams.ActualWL,
		PWQ:  d.FixedParams.PulseWidthNo,
		FLS:  d.FixedParams.FiberSpeed * 1000,
		PW:   d.FixedParams.PulseWidth,
		SQ:   d.FixedParams.SampleQTY,
		FLEN: d.TotalLength,
		BLV:  d.BellCoreVersion,
		KE:   len(d.Events),
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		log.Println("failed to create the html file")
	}
	htmlContent := buf.String()

	w.Write([]byte(htmlContent))
}

func (d *otdrRawData) return_index(loc float64) []float64 {
	closest := []float64{math.Inf(0), 0}

	for ind, i := range d.DataPoints {
		if i[0] == loc {
			return []float64{float64(ind), i[1]}
		}

		if math.Abs(loc-i[0]) < math.Abs(loc-closest[0]) && i[0] < loc {
			closest = []float64{float64(ind), i[1]}
		} else if i[0] > loc {
			break
		}
	}

	return []float64{closest[0], closest[1]}
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
		"YokogawaSpecial",
		"SetupParams",
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
		"AcqParam",
		"ViewParams",
		"SystemParams",
		"AnalysisParams",
		"MiscParams",
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

// getFixedParams function extracts the Fixed Parameters from the sor file and stores it in FixInfos struct.
func (d *otdrRawData) getFixedParams() {

	f := FixInfo{}

	if len(d.SecLocs["FxdParams"]) == 0 {
		log.Println("FxdParams section is missing")
		return
	}

	fixInfo := d.HexData[(d.SecLocs["FxdParams"][1]+10)*2 : (d.SecLocs[d.GetNext("FxdParams")][1] * 2)]
	p := 8

	f.DateTime = time.Unix(parsHexValue(fixInfo[:p]), 0)

	unit, err := hex.DecodeString(fixInfo[p : p+4])
	nukeIfErr(err)
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

	if len(d.SecLocs["DataPts"]) == 0 {
		log.Println("DataPts section is missing")
		return
	}
	dtpoints := d.HexData[d.SecLocs["DataPts"][1]*2 : d.SecLocs[d.GetNext("DataPts")][1]*2][40:]
	var start int64 = 0
	var cumulative_length float64 = 0

	for i := range d.FixedParams.SampleQTY {

		qty := int64(d.FixedParams.SampleQTY[i])
		resolution := d.FixedParams.Resolution[i]

		var j int64
		for j = 0; j < qty; j++ {
			index := start + j
			if index < int64(len(dtpoints)) {
				hex_value := dtpoints[index*4 : index*4+4]
				db_value := math.Round(dB(parsHexValue(hex_value))*1000) / 1000
				passedlen := math.Round(float64(cumulative_length*1000)) / 1000
				dataPoint := []float64{passedlen, db_value}
				d.DataPoints = append(d.DataPoints, dataPoint)
				cumulative_length += resolution
			}
		}
		start += qty
	}
}

// SupParams function extracts the Supplier Parameters from the sor file and stores it in SupParam struct.
func (d *otdrRawData) getSupParams() {

	if len(d.SecLocs["SupParams"]) == 0 {
		log.Println("SupParams section is missing")
		return
	}

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

func (d *otdrRawData) extractData(data []string, item int) string {
	if len(data)-1 < item {
		return ""
	} else {
		return strings.TrimSpace(data[item])
	}
}

// GenParams function extracts the General Parameters from the sor file and stores it in GenParam struct.
func (d *otdrRawData) getGenParams() {

	if len(d.SecLocs["GenParams"]) == 0 {
		log.Println("GenParams section is missing")
		return
	}

	genStringBeforeSplit := strings.Split(d.Decodedfile[d.SecLocs["GenParams"][1]+10:d.SecLocs[d.GetNext("GenParams")][1]], "\x00")
	genString := genStringBeforeSplit[:len(genStringBeforeSplit)-1]

	genInfo := GenParam{
		CableID:        d.extractData(genString, 0)[2:],
		Lang:           d.extractData(genString, 0)[:2],
		FiberID:        d.extractData(genString, 1),
		LocationA:      d.extractData(genString, 2)[4:],
		LocationB:      d.extractData(genString, 3),
		CableCode:      d.extractData(genString, 4),
		BuildCondition: d.extractData(genString, 5),
		Operator:       d.extractData(genString, 13),
		Comment:        d.extractData(genString, 14),
		FiberType:      "G." + strconv.FormatInt(parsHexValue(hex.EncodeToString([]byte(d.extractData(genString, 2)[:2]))), 10),
		OTDRWavelength: strconv.FormatInt(parsHexValue(hex.EncodeToString([]byte(d.extractData(genString, 2)[2:4]))), 10) + " nm",
	}

	d.GenParams = genInfo
}

// getFiberLength calculates the fiber length and returns it.
func (d *otdrRawData) getFiberLength() {
	for _, v := range d.Events {
		if strings.Contains(v.EventType, "EXX") || strings.Contains(v.EventType, "E99") {
			d.TotalLength = float64(v.EventLocM)
		}
	}
}

// getBellCoreVersion reads the bellcore version from the sor file and returns it.
func (d *otdrRawData) getBellCoreVersion() {

	if len(d.SecLocs["Map"]) == 0 {
		log.Println("Map section is missing")
		return
	}
	d.BellCoreVersion = float64(parsHexValue(d.HexData[(d.SecLocs["Map"][0]+4)*2:(d.SecLocs["Map"][0]+5)*2])) / 100.0
}

// getTotalLoss reads the total loss of the fiber from the sor file and returns it.
func (d *otdrRawData) getTotalLoss() {

	if len(d.SecLocs["WaveMTSParams"]) > 0 {
		totallossinfo := d.HexData[(d.SecLocs["WaveMTSParams"][1]-22)*2 : (d.SecLocs["WaveMTSParams"][1]-18)*2]
		d.TotalLoss = float64(parsHexValue(totallossinfo)) * 0.001
	} else {
		d.TotalLoss = 0
	}
}

// getKeyEvents function extracts the events information from the sor file and stores it in OTDREvent struct.
func (d *otdrRawData) getKeyEvents() {

	d.Events = map[int]OTDREvent{}

	if len(d.SecLocs["KeyEvents"]) == 0 {
		log.Println("KeyEvents section is missing")
		return
	}
	events := d.HexData[(d.SecLocs["KeyEvents"][1]+10)*2 : d.SecLocs[d.GetNext("KeyEvents")][1]*2]
	p := d.mapKeyEvents(events)

	var eventhexlist []string
	for _, v := range p {
		eventhexlist = append(eventhexlist, events[v[0]:v[1]])
	}

	for _, e := range eventhexlist {

		event := OTDREvent{}
		eNum := int(parsHexValue(e[:4]))
		event.EventNumber = eNum

		event.EventLocM = float64(parsHexValue(e[4:12])) * (math.Pow(10, -4)) * float64(d.FixedParams.FiberSpeed)

		stValue := mod(event.EventLocM, d.FixedParams.Resolution[0])
		if stValue >= d.FixedParams.Resolution[0]/2 {
			event.EventLocM = event.EventLocM + (d.FixedParams.Resolution[0] - stValue)
		} else {
			event.EventLocM = event.EventLocM + -stValue
		}

		event.EventLocM = math.Round(event.EventLocM*1000) / 1000

		event.Slope = float64(parsHexValue(e[12:16])) * 0.001
		event.SpliceLoss = float64(parsHexValue(e[16:20])) * 0.001
		if parsHexValue(e[20:28]) > 0 {
			event.RefLoss = float64((float64(parsHexValue(e[20:28])) - math.Pow(2, 32)) * 0.001)
		} else {
			event.RefLoss = float64(parsHexValue(e[20:28]))
		}

		eventType, err := hex.DecodeString(e[28:44])
		nukeIfErr(err)
		event.EventType = string(eventType)
		event.EndOfPreviousEvent = int(parsHexValue(e[44:52]))
		event.BegOfCurrentEvent = int(parsHexValue(e[52:60]))
		event.EndOfCurrentEvent = int(parsHexValue(e[60:68]))
		event.BegOfNextEvent = int(parsHexValue(e[68:76]))
		event.PeakCurrentEvent = int(parsHexValue(e[76:84]))
		if len(e) > 88 {
			if len(e) < 102 {
				comment, err := hex.DecodeString(e[84:])
				nukeIfErr(err)
				event.Comment = string(comment)
			} else {
				comment, err := hex.DecodeString(e[84:102])
				nukeIfErr(err)
				event.Comment = string(comment)
			}
		}
		d.Events[eNum] = event
	}
}

func (d *otdrRawData) export2Json() {

	var exportData = struct {
		Filename        string            `json:"File Name"`
		FixedParams     FixInfo           `json:"Fixed Parameters"`
		TotalLoss       float64           `json:"Total Fiber Loss(dB)"`
		TotalLength     float64           `json:"Fiber Length(km)"`
		GenParams       GenParam          `json:"General Information"`
		Supplier        SupParam          `json:"Supplier Information"`
		Events          map[int]OTDREvent `json:"Key Events"`
		BellCoreVersion float64           `json:"Bellcore Version"`
	}{
		Filename:        d.Filename,
		FixedParams:     d.FixedParams,
		TotalLoss:       d.TotalLoss,
		TotalLength:     d.TotalLength,
		GenParams:       d.GenParams,
		Supplier:        d.Supplier,
		Events:          d.Events,
		BellCoreVersion: d.BellCoreVersion,
	}

	b, err := json.MarshalIndent(exportData, "", "  ")
	nukeIfErr(err)
	_ = os.WriteFile("OTDR_Output.json", b, 0644)
	fmt.Println("Json file has been exported! - json file name: OTDR_Output.json")
}

func getCliArgs() map[string]*string {

	m := map[string]*string{}

	filePath := flag.String("sorfile", "", "Mandatory - Path to the sor file")
	flag.StringVar(filePath, "s", "", "Mandatory - Path to the sor file")
	m["filePath"] = filePath

	draw := flag.String("draw", "yes", "Optional - whether to draw the graph or not, yes , no")
	flag.StringVar(draw, "d", "yes", "Optional - whether to draw the graph or not, yes , no")
	m["draw"] = draw

	json := flag.String("json", "yes", "Optional - whether to dump as json or not, yes , no")
	flag.StringVar(json, "j", "yes", "Optional - whether to dump as json or not, yes , no")
	m["json"] = json

	flag.Parse()

	if len(*filePath) == 0 {
		log.Fatalln("no file has been specified")
	}

	return m
}

func ParseOTDRFile(args map[string]*string) {

	d := ReadSorFile(*args["filePath"])
	d.GetOrder()
	d.getBellCoreVersion()
	d.getTotalLoss()
	d.getSupParams()
	d.getGenParams()
	d.getFixedParams()
	d.getDataPoints()
	d.getKeyEvents()
	d.getFiberLength()

	if strings.EqualFold(*args["json"], "yes") {
		d.export2Json()
	}

	//Draw the graph
	if strings.EqualFold(*args["draw"], "yes") {

		d.draw()
	}
}

func main() {

	defer customPanicHandler()

	args := getCliArgs()
	ParseOTDRFile(args)
}
