/*
Author: Naseredin Aramnejad naseredin.aramnejad@gmail.com
This script is designed to extract all the possible information from the
given sor file.
each sor file (Provided by OTDR Equipment) contains multiple data blocks
and since it's a binary file, it should be red per byte.

Formulas and blueprint of this script are inspired by the information provided by:
Sidney Li
http://morethanfootnotes.blogspot.com/2015/07/the-otdr-optical-time-domain.html
*/
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

/*
Speed of light in vaccuum. It's used to calculate lightspeed in the
fiber medium , Fiber length and scan resolution.
*/
const lightSpeed = 299.79181901 // m/µsec

//This is the struct wrapping all the extracted information and being exported as JSON
type otdrData struct {
	Supplier        supPram
	GenInfo         genParams `json:"General Information"`
	Events          []otdrEvent
	FixInfo         fixInfos `json:"Fixed Parameters"`
	FiberLength_km  float32  `json:"Fiber Length(km)"`
	BellCoreVersion float32  `json:"Bellcore Version"`
	TotalLoss_dB    float32  `json:"Total Fiber Loss(dB)"`
	AvgLossPerKm    float32  `json:"Average Loss per Km(dB)"`
}

//Supplier Params extracted from the sor file
type supPram struct {
	OtdrSupplier   string `json:"OTDR Supplier"`
	OtdrName       string `json:"OTDR Name"`
	OtdrSN         string `json:"OTDR SN"`
	OtdrModuleName string `json:"OTDR Module Name"`
	OtdrModuleSN   string `json:"OTDR Module SN"`
	OtdrSwVersion  string `json:"OTDR SW Version"`
	OtdrOtherInfo  string `json:"OTDR Other Info"`
}

//General Params extracted from the sor file
type genParams struct {
	CableId        string `json:"Cable Id"`
	FiberId        string `json:"Fiber Id"`
	LocationA      string
	LocationB      string
	BuildCondition string `json:"Build Condition"`
	Comment        string
	CableCode      string `json:"Cable Code"`
	Operator       string
	FiberType      string `json:"Fiber Type"`
	OtdrWavelength string `json:"OTDR Wavelength"`
}

//Event information extracted from the sor file
type otdrEvent struct {
	EventType               string  `json:"Event Type"`
	EventPoint_m            float32 `json:"Event Point(m)"`
	EventNumber             int     `json:"Event Number"`
	Slope                   float32 `json:"Slope(dB)"`
	SpliceLoss_dB           float32 `json:"Splice Loss(dB)"`
	ReflectionLoss_dB       float32 `json:"Reflection Loss(dB)"`
	EndOfPreviousEvent      int     `json:"Previous Event-End"`
	BegOfCurrentEvent       int     `json:"Current Event-Start"`
	EndOfCurrentEvent       int     `json:"Current Event-End"`
	BegOfNextEvent          int     `json:"Next Event-Start"`
	PeakpointInCurrentEvent int     `json:"Peak point"`
}

//Fixed Params extracted from the sor file
type fixInfos struct {
	DateTime              time.Time
	Unit                  string
	ActualWavelength_nm   float32 `json:"Actual Wavelength"`
	PulseWidthNo          int64   `json:"Pulse Width No"`
	PulseWidth_ns         int64   `json:"Pulse Width(ns)"`
	SampleQty             int64   `json:"Sample Quantity"`
	Ior                   int64
	RefractionIndex       float32 `json:"Refraction Index"`
	FiberLightSpeed_km_ms float32 `json:"Fiber Light Speed"`
	Resolution_m          float32 `json:"RScan Resolution"`
	BackscatteringCo_dB   float32 `json:"Back-Scattering"`
	Averaging             int64
	AveragingTime_M       float32 `json:"Averaging Time"`
	Range_km              float32 `json:"Scan Range"`
}

func errDealer(err error) {
	if err != nil {
		panic(err)
	}
}

/*
	This function opens the sor file and returns a hex string (hexData)
	and a text string (charString) from the file to the main function.
	Basically reading the whole file and putting it in RAM
*/
func readSorFile(filename string) (string, string) {

	var fileInArray []byte
	var hexData string

	f, err := os.Open(filename)
	errDealer(err)

	defer f.Close()

	for {
		eachByte := make([]byte, 1)
		_, err = f.Read(eachByte)

		if err == io.EOF {
			break
		}
		errDealer(err)

		fileInArray = append(fileInArray, eachByte...)
	}
	//Converting the byte array into a hex String
	hexData = hex.EncodeToString(fileInArray)

	//Converting the HexData to a text string
	chars, err := hex.DecodeString(hexData)
	errDealer(err)

	charString := string(chars)
	return hexData, charString
}

func reverse(s string) string {
	str := ""
	for ind := 0; ind < len(s); ind += 2 {
		str = s[ind:ind+2] + str
	}
	return str
}

func hexParser(hexData string) int64 {
	output, err := strconv.ParseInt(reverse(hexData), 16, 64)
	if err != nil {
		fmt.Println(err)
		return 0
	}
	return output
}

func fixedParams(hexData, charString string) fixInfos {

	fixInfo := hexData[(strings.Index(charString[224:], "FxdParams")+234)*2 : (strings.Index(charString[224:], "DataPts")+224)*2]

	unit, err := hex.DecodeString(fixInfo[8:12])
	errDealer(err)

	ior := hexParser(fixInfo[56:64])
	refractionIndex := float32(ior) * float32(math.Pow(10, -5))
	fiberLightSpeed_km_ms := lightSpeed / refractionIndex
	resolution_m := float32(hexParser(fixInfo[40:48])) * float32(math.Pow(10, -8)) * fiberLightSpeed_km_ms
	sampleQty := hexParser(fixInfo[48:56])

	fixParam := fixInfos{
		DateTime:              time.Unix(hexParser(fixInfo[:8]), 0),
		Unit:                  string(unit),
		ActualWavelength_nm:   float32(hexParser(fixInfo[12:16])) / 10.0,
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
		Range_km:              float32(sampleQty) * resolution_m,
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

func fiberLength(hexData, charString string, fixParams fixInfos) float32 {
	length := float32(hexParser(hexData[(strings.Index(charString[224:], "WaveMTSParams")+210)*2 : (strings.Index(charString[224:], "WaveMTSParams")+214)*2]))
	return (length * float32(math.Pow(10, -4)) * lightSpeed / fixParams.RefractionIndex) / 1000
}

func bellCoreVersion(hexData, charString string) float32 {
	return float32(hexParser(hexData[(strings.Index(charString, "Map")+4)*2:(strings.Index(charString, "Map")+5)*2])) / 100.0
}

func totalLoss(hexData, charString string) float32 {
	totallossinfo := hexData[(strings.Index(charString[224:], "WaveMTSParams")+202)*2 : (strings.Index(charString[224:], "WaveMTSParams")+206)*2]
	return float32(hexParser(totallossinfo)) * 0.001
}

func keyEvents(hexData string, charString string, fiberLightSpeed_km_ms float32, resolution_m float32) []otdrEvent {
	var keyevents []otdrEvent
	events := hexData[(strings.Index(charString[224:], "KeyEvents")+224)*2 : (strings.Index(charString[224:], "WaveMTSParams")+224)*2]
	evnumbers := hexParser(events[20:24])
	var eventhex = []string{}
	for ind := 0; ind < int(88*evnumbers); ind += 88 {
		eventhex = append(eventhex, events[24:][ind:ind+88])
	}

	for e := range eventhex {
		eventType, err := hex.DecodeString(eventhex[e][28:44])
		errDealer(err)

		eventPoint_m := float32(hexParser(eventhex[e][4:12])) * float32(math.Pow(10, -4)) * fiberLightSpeed_km_ms
		stValue := float32(math.Mod(float64(eventPoint_m), float64(resolution_m)))
		if stValue >= (resolution_m / 2.0) {
			eventPoint_m = eventPoint_m + resolution_m - stValue
		} else {
			eventPoint_m = eventPoint_m - stValue
		}

		var reflectionLoss_dB float32
		if hexParser(eventhex[e][20:28]) > 0 {
			reflectionLoss_dB = float32((float64(hexParser(eventhex[e][20:28])) - math.Pow(2, 32)) * 0.001)
		} else {
			reflectionLoss_dB = float32(hexParser(eventhex[e][20:28]))
		}

		eventEntry := otdrEvent{
			EventNumber:             int(hexParser(eventhex[e][:4])),
			EventPoint_m:            eventPoint_m,
			Slope:                   float32(hexParser(eventhex[e][12:16])) * 0.001,
			SpliceLoss_dB:           float32(hexParser(eventhex[e][16:20])) * 0.001,
			ReflectionLoss_dB:       reflectionLoss_dB,
			EventType:               string(eventType),
			EndOfPreviousEvent:      int(hexParser(eventhex[e][44:52])),
			BegOfCurrentEvent:       int(hexParser(eventhex[e][52:60])),
			EndOfCurrentEvent:       int(hexParser(eventhex[e][60:68])),
			BegOfNextEvent:          int(hexParser(eventhex[e][68:76])),
			PeakpointInCurrentEvent: int(hexParser(eventhex[e][76:84])),
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
	errDealer(err)
	_ = ioutil.WriteFile("otdr_Output.json", b, 0644)
	fmt.Println("Json file has been exported! - json file name: otdr_Output.json")
}

func main() {

	//Reading the Sor file by entering it's path
	var sorFileName string
	fmt.Print("Enter the .sor filename: ")
	fmt.Scanln(&sorFileName)

	/*
		Invoking the filereader function, the result will be
		a hex version of the sor file (hexData)and a text encoded
		version of the sor file (charString)hexData will be
		used to extract the numeric values while charString is used
		to find the related sections and also for text data extraction.
	*/
	hexData, charString := readSorFile(sorFileName)

	//###### Extracting the information by invoking the corresponding function #####

	/*
		This function call will extract the information in the
		fixParam part of the sor file and store them in the type fixInfos struct
		Since we need some of the fixedParams to calculate other params,It's
		called separately unlike other extractions.
	*/
	fixed := fixedParams(hexData, charString)

	loss := totalLoss(hexData, charString)
	length := fiberLength(hexData, charString, fixed)

	// otdrData is the main struct which contains and gather all the extracted information to be converted into JSON format
	otdrExtractedData := otdrData{
		FixInfo:         fixed,                                                                           //Fixed params that are extracted above.
		Supplier:        supParams(charString),                                                           //OTDR Module Supplier Information
		GenInfo:         genParam(charString),                                                            //OTDR scan General Information
		Events:          keyEvents(hexData, charString, fixed.FiberLightSpeed_km_ms, fixed.Resolution_m), //OTDR scan key events like loss events and reflection events
		TotalLoss_dB:    loss,                                                                            //Total fiber loss value of the scan, end to end in dB
		FiberLength_km:  length,                                                                          //Total fiber length, end to end
		BellCoreVersion: bellCoreVersion(hexData, charString),                                            //Bellcore version of the file, 2.1 is supported by this script
		AvgLossPerKm:    loss / length,
	}

	jsonExport(otdrExtractedData)
}
