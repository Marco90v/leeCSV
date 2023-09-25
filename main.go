// main3.go
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
	"sync"
)

// nacionalidad;cedula;primer_apellido;segundo_apellido;primer_nombre;segundo_nombre;cod_centro
type Record struct {
	Nacionalidad     string
	DNI           string
	Primer_Apellido  string
	Segundo_Apellido string
	Primer_Nombre    string
	Segundo_Nombre   string
	Cod_Centro       string
}

type MyError struct{}

const CSV string = "/home/i320/Documentos/nacional.csv"

var DNI string
var primerNombre string
var segundoNombre string
var primerApellido string
var segundoApellido string

var varParams = Record{
	DNI: "",
	Primer_Nombre: "",
	Segundo_Nombre: "",
	Primer_Apellido: "",
	Segundo_Apellido: "",
}

func getParams(){
	tempDNI := flag.String("DNI", "", "Documento de identidad")
	tempPrimerNombre := flag.String("primerNombre", "", "Primer Nombre")
	tempSegundoNombre := flag.String("segundoNombre", "", "Segungo Nombre")
	tempPrimerApellido := flag.String("primerApellido", "", "Primer Apellido")
	tempSegundoApellido := flag.String("segundoApellido", "", "Segundo Apellido")

	flag.Parse()
	varParams.DNI = *tempDNI
	varParams.Primer_Nombre = *tempPrimerNombre
	varParams.Segundo_Nombre = *tempSegundoNombre
	varParams.Primer_Apellido = *tempPrimerApellido
	varParams.Segundo_Apellido = *tempSegundoApellido
}


func main() {
	fmt.Println("Iniciando busqueda")
	getParams()
	nmWp := 4

	input := make(chan []string, nmWp)
	output := make(chan string)
	var wg sync.WaitGroup
	
	records, err := readFile(CSV)
	if err != nil {
		log.Fatal(err)
	}
	worker := func(jobs <-chan []string, results chan<- string) {
		for {
			select {
			case job, ok := <-jobs: // you must check for readable state of the channel.
				if !ok {
					return
				}
				user := parseStruct(job)

				//menos esficiente
				//*************************************
					// o1 := make(chan string)
					// o2 := make(chan string)
					// o3 := make(chan string)
					// o4 := make(chan string)
					// o5 := make(chan string)

					// go hiloValDNI(user, user, o1)
					// go hiloValPrimerNombre(user, user, o2)
					// go hiloValSegundoNombre(user, user, o3)
					// go hiloValPrimerApellido(user, user, o4)
					// go hiloValSegundoApellido(user, user, o5)

					// var h Record
					// h.DNI = <-o1
					// h.Primer_Nombre = <-o2
					// h.Segundo_Nombre = <-o3
					// h.Primer_Apellido = <-o4
					// h.Segundo_Apellido = <-o5

					// // fmt.Println(h)

					// var yes bool = true
					// values := reflect.ValueOf(h)
					// for i := 0; i < values.NumField(); i++ {
					// 	if (values.Field(i).String() == "nil"){
					// 		yes = false
					// 	}
					// }
					// if yes{
					// 	var cadena = fmt.Sprintf("%s - %s %s, %s %s",
					// 		h.DNI,
					// 		h.Primer_Nombre,
					// 		h.Segundo_Nombre,
					// 		h.Primer_Apellido,
					// 		h.Segundo_Apellido,
					// 	)
					// 	results <-cadena
					// }
				//**************************************

				//Mas esficiente
				//******************
				var yes bool = true
				values := reflect.ValueOf(*user)
				values2 := reflect.ValueOf(varParams)
				for i := 1; i <= values.NumField()-2; i++ {
					if (values2.Field(i).String() != "" && values.Field(i).String() != values2.Field(i).String() ){
						yes = false
						break
					}
				}
				if yes {
					var cadena = fmt.Sprintf("%s - %s %s, %s %s",
						user.DNI,
						user.Primer_Nombre,
						user.Segundo_Nombre,
						user.Primer_Apellido,
						user.Segundo_Apellido,
					)
					results <-cadena
				}
				//*********************

			}
		}
	}

	for w := 0; w < nmWp; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker(input, output)
		}()
	}

	go readRecords(records, input)
	go closeResult(&wg, output)

	for user := range output {
		println(user)
	}
	fmt.Println("Finalizando busqueda")

}


func hiloValDNI(cadena *Record, user *Record, results chan<- string){
	// fmt.Println(user.DNI == "")
	if(varParams.DNI == ""){
		results <-user.DNI
	}else if(user.DNI == varParams.DNI){
		results <-cadena.DNI
	}else{
		results <-"nil"
	}
	// if(user.DNI == DNI){
	// 	results <-cadena.DNI
	// }
}

func hiloValPrimerNombre(cadena *Record, user *Record, results chan<- string){
	if(varParams.Primer_Nombre == ""){
		results <-user.Primer_Nombre
	}else if(user.Primer_Nombre == varParams.Primer_Nombre){
		results <-cadena.Primer_Nombre
	}else{
		results <-"nil"
	}
	// if(user.Primer_Nombre == primerNombre){
	// 	results <-cadena
	// }
}

func hiloValSegundoNombre(cadena *Record, user *Record, results chan<- string){
	if(varParams.Segundo_Nombre == ""){
		results <-user.Segundo_Nombre
	}else if(user.Segundo_Nombre == varParams.Segundo_Nombre){
		results <-cadena.Segundo_Nombre
	}else{
		results <-"nil"
	}
	// if(user.Segundo_Nombre == segundoNombre){
	// 	results <-cadena
	// }
}

func hiloValPrimerApellido(cadena *Record, user *Record, results chan<- string){
	if(varParams.Primer_Apellido == ""){
		results <-user.Primer_Apellido
	}else if(user.Primer_Apellido == varParams.Primer_Apellido){
		results <-cadena.Primer_Apellido
	}else{
		results <-"nil"
	}
	// if(user.Primer_Apellido == primerApellido){
	// 	results <-cadena
	// }
}

func hiloValSegundoApellido(cadena *Record, user *Record, results chan<- string){
	if(varParams.Segundo_Apellido == ""){
		results <-user.Segundo_Apellido
	}else if(user.Segundo_Apellido == varParams.Segundo_Apellido){
		results <-cadena.Segundo_Apellido
	}else{
		results <-"nil"
	}
	// if(user.Segundo_Apellido == segundoApellido){
	// 	results <-cadena
	// }
}

func readRecords(records *csv.Reader, jobs chan<- []string) {
	for {
		record, err := records.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		jobs <- record
	}
	close(jobs)
}

// func closeResult(wg *sync.WaitGroup, result chan *Record) {
func closeResult(wg *sync.WaitGroup, result chan string) {
	wg.Wait()
	close(result)
}

func parseStruct(record []string) *Record {
	user := &Record{
		// Nacionalidad:     record[0],
		DNI:           record[1],
		Primer_Nombre:    record[4],
		Segundo_Nombre:   record[5],
		Primer_Apellido:  record[2],
		Segundo_Apellido: record[3],
		// Cod_Centro:       record[6],
	}
	return user
}

func readFile(CSV string) (*csv.Reader, error) {
	// Open the CSV file for appending
	file, err := os.Open(CSV)
	if err != nil {
		panic(err)
	}

	r := csv.NewReader(file)
	r.Comma = ';'
	r.Comment = '#'

	// skip first line
	if _, err := r.Read(); err != nil {
		return r, err
	}
	return r, nil
}
