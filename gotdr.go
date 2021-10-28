//OTDR Reader. Detailed comments will be added later!

package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

const lightSpeed = 299.79181901

type otdrData struct {
	Supplier        supPram
	GenInfo         genParams
	Events          []otdrEvent
	FixInfo         fixInfos
	FiberLength_m   float64
	BellCoreVersion float32
	TotalLoss_dB    string
}

type supPram struct {
	OtdrSupplier   string
	OtdrName       string
	OtdrSN         string
	OtdrModuleName string
	OtdrModuleSN   string
	OtdrSwVersion  string
	OtdrOtherInfo  string
}

type genParams struct {
	CableId        string
	FiberId        string
	LocationA      string
	LocationB      string
	BuildCondition string
	Comment        string
	CableCode      string
	Operator       string
	FiberType      string
	OtdrWavelength string
}

type otdrEvent struct {
	EventType               string
	EventPoint_m            float64
	EventNumber             int
	Slope                   float64
	SpliceLoss_dB           float64
	ReflectionLoss_dB       float64
	EndOfPreviousEvent      int64
	BegOfCurrentEvent       int64
	EndOfCurrentEvent       int64
	BegOfNextEvent          int64
	PeakpointInCurrentEvent int64
}

type fixInfos struct {
	DateTime              time.Time
	Unit                  string
	ActualWavelength_nm   float64
	PulseWidthNo          int64
	PulseWidth_ns         int64
	SampleQty             int64
	Ior                   int64
	RefractionIndex       float64
	FiberLightSpeed_km_ms float64
	Resolution_m          float64
	BackscatteringCo_dB   float32
	Averaging             int64
	AveragingTime_M       float32
	Range_km              float64
}

//This function get's the sor file path, opens it and retuns a byte array from the file to the main function
func readSorFile(filename string) (string, string) {

	var fileInArray []byte
	var hexData string

	f, e := os.Open(filename)
	if e != nil {
		panic(e)
	}
	defer f.Close()

	for {
		b := make([]byte, 1)
		_, e = f.Read(b)

		if e == io.EOF {
			break
		} else if e != nil {
			panic(e)
		}
		fileInArray = append(fileInArray, b...)
	}

	hexData = hex.EncodeToString(fileInArray)
	chars, err := hex.DecodeString(hexData)

	if err != nil {
		panic(err)
	}

	charString := string(chars)
	return hexData, charString
}

func Reverse(s string) string {
	str := ""
	for ind := 0; ind < len(s); ind += 2 {
		str = s[ind:ind+2] + str
	}
	return str
}

func hexParser(hexData string) int64 {
	output, err := strconv.ParseInt(Reverse(hexData), 16, 64)
	if err != nil {
		fmt.Println(err)
		return 0
	}
	return output
}

func fixedParams(hexData, charString string) fixInfos {

	fixInfo := hexData[(strings.Index(charString[224:], "FxdParams")+234)*2 : (strings.Index(charString[224:], "DataPts")+224)*2]
	unit, err := hex.DecodeString(fixInfo[8:12])
	if err != nil {
		fmt.Println(err)
	}
	ior := hexParser(fixInfo[56:64])
	refractionIndex := float64(ior) * math.Pow(10, -5)
	fiberLightSpeed_km_ms := lightSpeed / refractionIndex
	resolution_m := float64(hexParser(fixInfo[40:48])) * math.Pow(10, -8) * fiberLightSpeed_km_ms
	sampleQty := hexParser(fixInfo[48:56])

	fixParam := fixInfos{
		DateTime:              time.Unix(hexParser(fixInfo[:8]), 0),
		Unit:                  string(unit),
		ActualWavelength_nm:   float64(hexParser(fixInfo[12:16])) / 10.0,
		PulseWidthNo:          hexParser(fixInfo[32:36]),
		PulseWidth_ns:         hexParser(fixInfo[36:40]),
		SampleQty:             sampleQty,
		Ior:                   ior,
		RefractionIndex:       refractionIndex,
		FiberLightSpeed_km_ms: fiberLightSpeed_km_ms,
		Resolution_m:          resolution_m,
		BackscatteringCo_dB:   float32(hexParser(fixInfo[64:68])) * -0.1,
		Averaging:             hexParser(fixInfo[68:76]),
		AveragingTime_M:       float32(hexParser(fixInfo[76:80])) / 600,
		Range_km:              float64(sampleQty) * resolution_m,
	}

	return fixParam
}

func supParams(charString string) supPram {
	supString := charString[strings.Index(charString[224:], "SupParams")+234 : strings.Index(charString[224:], "FxdParams")+224]
	slicedparams := strings.Split(supString, "\x00")[:7]

	supInfo := supPram{
		OtdrSupplier:   strings.TrimSpace(slicedparams[0]),
		OtdrName:       strings.TrimSpace(slicedparams[1]),
		OtdrSN:         strings.TrimSpace(slicedparams[2]),
		OtdrModuleName: strings.TrimSpace(slicedparams[3]),
		OtdrModuleSN:   strings.TrimSpace(slicedparams[4]),
		OtdrSwVersion:  strings.TrimSpace(slicedparams[5]),
		OtdrOtherInfo:  strings.TrimSpace(slicedparams[6]),
	}

	return supInfo
}

func genParam(charString string) genParams {

	genString := charString[strings.Index(charString[224:], "GenParams")+234 : strings.Index(charString[224:], "SupParams")+224]
	slicedparams := strings.Split(genString, "\x00")

	genInfo := genParams{
		CableId:        strings.TrimSpace(slicedparams[0]),
		FiberId:        strings.TrimSpace(slicedparams[1]),
		LocationA:      strings.TrimSpace(slicedparams[2][4:]),
		LocationB:      strings.TrimSpace(slicedparams[3]),
		CableCode:      strings.TrimSpace(slicedparams[4]),
		BuildCondition: strings.TrimSpace(slicedparams[5]),
		Operator:       strings.TrimSpace(slicedparams[13]),
		Comment:        strings.TrimSpace(slicedparams[14]),
		FiberType:      "G." + strconv.FormatInt(hexParser(hex.EncodeToString([]byte(slicedparams[2][:2]))), 10),
		OtdrWavelength: strconv.FormatInt(hexParser(hex.EncodeToString([]byte(slicedparams[2][2:4]))), 10) + " nm",
	}
	return genInfo
}

func fiberLength(hexData, charString string, fixParams fixInfos) float64 {
	length := float64(hexParser(hexData[(strings.Index(charString[224:], "WaveMTSParams")+210)*2 : (strings.Index(charString[224:], "WaveMTSParams")+214)*2]))
	return length * math.Pow(10, -4) * lightSpeed / fixParams.RefractionIndex
}

func bellCoreVersion(hexData, charString string) float32 {
	return float32(hexParser(hexData[(strings.Index(charString, "Map")+4)*2:(strings.Index(charString, "Map")+5)*2])) / 100.0
}

func totalLoss(hexData, charString string) string {
	totallossinfo := hexData[(strings.Index(charString[224:], "WaveMTSParams")+202)*2 : (strings.Index(charString[224:], "WaveMTSParams")+206)*2]
	return fmt.Sprintf("%f", float64(hexParser(totallossinfo))*0.001)
}

func dataPts(hexData string, charString string, sampleQty int64, resolution_m float64) map[float64]float64 {
	data := hexData[(strings.Index(charString[224:], "DataPts")+224)*2 : (strings.Index(charString[224:], "KeyEvents")+224)*2][40:]
	dataset := make(map[float64]float64)
	for length := 0; length < int(sampleQty); length++ {
		passedlen := float64(length) * resolution_m
		dataset[passedlen] = float64(hexParser(data[length*4:length*4+4])) * -1000.0 * math.Pow(10, -6)
	}
	return dataset
}

func keyEvents(hexData string, charString string, fiberLightSpeed_km_ms float64, resolution_m float64) []otdrEvent {
	var keyevents []otdrEvent
	events := hexData[(strings.Index(charString[224:], "KeyEvents")+224)*2 : (strings.Index(charString[224:], "WaveMTSParams")+224)*2]
	evnumbers := hexParser(events[20:24])
	var eventhex = []string{}
	for ind := 0; ind < int(88*evnumbers); ind += 88 {
		eventhex = append(eventhex, events[24:][ind:ind+88])
	}

	for e := range eventhex {
		eventType, err := hex.DecodeString(eventhex[e][28:44])
		if err != nil {
			fmt.Println(err)
		}
		eventPoint_m := float64(hexParser(eventhex[e][4:12])) * math.Pow(10, -4) * fiberLightSpeed_km_ms
		stValue := math.Mod(eventPoint_m, resolution_m)
		if stValue >= (resolution_m / 2.0) {
			eventPoint_m = eventPoint_m + resolution_m - stValue
		} else {
			eventPoint_m = eventPoint_m - stValue
		}

		var reflectionLoss_dB float64
		if hexParser(eventhex[e][20:28]) > 0 {
			reflectionLoss_dB = (float64(hexParser(eventhex[e][20:28])) - math.Pow(2, 32)) * 0.001
		} else {
			reflectionLoss_dB = float64(hexParser(eventhex[e][20:28]))
		}

		eventEntry := otdrEvent{
			EventNumber:             int(hexParser(eventhex[e][:4])),
			EventPoint_m:            eventPoint_m,
			Slope:                   float64(hexParser(eventhex[e][12:16])) * 0.001,
			SpliceLoss_dB:           float64(hexParser(eventhex[e][16:20])) * 0.001,
			ReflectionLoss_dB:       reflectionLoss_dB,
			EventType:               string(eventType),
			EndOfPreviousEvent:      hexParser(eventhex[e][44:52]),
			BegOfCurrentEvent:       hexParser(eventhex[e][52:60]),
			EndOfCurrentEvent:       hexParser(eventhex[e][60:68]),
			BegOfNextEvent:          hexParser(eventhex[e][68:76]),
			PeakpointInCurrentEvent: hexParser(eventhex[e][76:84]),
		}

		keyevents = append(keyevents, eventEntry)
		if string(eventEntry.EventType[1]) == "E" {
			break
		}
	}

	return keyevents
}

func jsonExport(data otdrData) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(b))
	_ = ioutil.WriteFile("otdr_Output.json", b, 0644)
	fmt.Println("Json file has been exported!")
}

// func plotter(data otdrData) {

// 	// c = plt.subplots(figsize=(13,8))[1]
// 	// c.plot(self.dataset.keys(),self.dataset.values(),color='tab:green',lw=0.6)

// 	for ev := range data.Events {
// 		refQ := ""
// 		lossQ := ""
// 		tmp1 := data.Events[ev][EventPoint_m]
// 		tmp2 := data.Events[ev]

// 		if float64(tmp2[ReflectionLoss_dB]) == 0 {
// 			eventType := "Splice"
// 			refQ = " - OK"
// 		} else {
// 			eventType = "Connector"
// 			if float64(tmp2[ReflectionLoss_dB]) <= -40 {
// 				refQ = " - OK"
// 			} else {
// 				refQ = " - !!!"
// 			}
// 		}
// 		if float64(tmp2[SpliceLoss_dB]) == 0 {
// 			lossQ = " - Ghost!"
// 		} else if float64(tmp2[SpliceLoss_dB]) <= 1 {
// 			lossQ = " - OK"
// 		} else {
// 			lossQ = " - !!!"
// 		}
// 		//c.annotate("",xy=(tmp1,self.dataset[tmp1] + 1),xytext=(tmp1,self.dataset[tmp1] - 1),arrowprops=dict(arrowstyle="<->",color="red",connectionstyle= "bar,fraction=0"))
// 		//c.annotate(f"  Event:{ev}\n  Type:  {eventType}\n  Len:   {round(tmp1,1)}m\n  Ref:   {tmp2['reflectionLoss_dB']}dB{refQ}\n  Loss:  {tmp2['spliceLoss_dB']}dB{lossQ}",xy=(tmp1,self.dataset[tmp1]),xytext=(tmp1,self.dataset[tmp1]-1))
// 	}
// 	// c.set(xlabel='Fiber Length (m)', ylabel='Optical Power(dB)',title='OTDR Graph')
// 	// plt.grid()
// 	// plt.show()
// }

func main() {

	//Reading the Sor file by entering it's path
	var sorFileName string
	fmt.Print("Enter the .sor filename: ")
	fmt.Scanln(&sorFileName)

	//Invoking the filereader function, the result will be []byte
	hexData, charString := readSorFile(sorFileName)

	//Extracting the information by invoking the corresponding function
	fixed := fixedParams(hexData, charString)
	otdrExtractedData := otdrData{
		FixInfo:         fixed,
		Supplier:        supParams(charString),
		GenInfo:         genParam(charString),
		Events:          keyEvents(hexData, charString, fixed.FiberLightSpeed_km_ms, fixed.Resolution_m),
		TotalLoss_dB:    totalLoss(hexData, charString),
		FiberLength_m:   fiberLength(hexData, charString, fixed),
		BellCoreVersion: bellCoreVersion(hexData, charString),
	}
	jsonExport(otdrExtractedData)
	// plotter(otdrExtractedData)
}

// datapoints := dataPts(hexData, charString, fixed.sampleQty, fixed.resolution_m)
// fmt.Println(datapoints)
