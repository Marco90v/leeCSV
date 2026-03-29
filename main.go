// main3.go
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"reflect"
	"sync"
)

// nacionalidad;cedula;primer_apellido;segundo_apellido;primer_nombre;segundo_nombre;cod_centro
type Record struct {
	Nacionalidad     string
	DNI              string
	Primer_Apellido  string
	Segundo_Apellido string
	Primer_Nombre    string
	Segundo_Nombre   string
	Cod_Centro       string
}

// const CSV string = "/home/i320/Documentos/nacional.csv"
const CSV string = "./nacional.csv"

var varParams = Record{
	DNI:              "",
	Primer_Nombre:    "",
	Segundo_Nombre:   "",
	Primer_Apellido:  "",
	Segundo_Apellido: "",
}

func getParams() {
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
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
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
				yes := true
				values := reflect.ValueOf(*user)
				values2 := reflect.ValueOf(varParams)
				for i := 1; i <= values.NumField()-2; i++ {
					if values2.Field(i).String() != "" && values.Field(i).String() != values2.Field(i).String() {
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
					results <- cadena
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

	errCh := make(chan error, 1)
	go readRecords(records, input, errCh)
	go closeResult(&wg, output)

	// Check for read errors
	select {
	case err := <-errCh:
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	default:
	}

	for user := range output {
		println(user)
	}
	fmt.Println("Finalizando busqueda")

}

// readRecords reads all records from the CSV and sends them to the jobs channel.
// It closes the jobs channel when done or on error.
func readRecords(records *csv.Reader, jobs chan<- []string, errCh chan<- error) {
	defer close(jobs)

	for {
		record, err := records.Read()
		if err == io.EOF {
			return
		}
		if err != nil {
			errCh <- fmt.Errorf("error reading CSV record: %w", err)
			return
		}
		jobs <- record
	}
}

// func closeResult(wg *sync.WaitGroup, result chan *Record) {
func closeResult(wg *sync.WaitGroup, result chan string) {
	wg.Wait()
	close(result)
}

func parseStruct(record []string) *Record {
	user := &Record{
		// Nacionalidad:     record[0],
		DNI:              record[1],
		Primer_Nombre:    record[4],
		Segundo_Nombre:   record[5],
		Primer_Apellido:  record[2],
		Segundo_Apellido: record[3],
		// Cod_Centro:       record[6],
	}
	return user
}

// readFile opens a CSV file and returns a reader configured for semicolon-separated files.
// It skips the header row. Returns an error if the file cannot be opened or read.
func readFile(path string) (*csv.Reader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file %s: %w", path, err)
	}

	r := csv.NewReader(file)
	r.Comma = ';'
	r.Comment = '#'

	// skip first line (header)
	if _, err := r.Read(); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	return r, nil
}
