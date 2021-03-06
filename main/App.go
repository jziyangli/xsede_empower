// Group Members: Evan Zhang, Alexander Garcia and Christina Monahan
// Distributed System for Tuning Hyperparameters of Neural Networks
// PAXOS election, consensus, and recovery algorithm

package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	/*"github.com/aunum/gold/pkg/v1/common/num"
	"github.com/aunum/gold/pkg/v1/common/require"
	"github.com/aunum/gold/pkg/v1/dense"
	mo "github.com/aunum/goro/pkg/v1/model"

	"gorgonia.org/tensor"*/

	//"test/go/pkg/mod/github.com/c-bata/goptuna@v0.8.1/successivehalving"

	"github.com/andlabs/ui"
	_ "github.com/andlabs/ui/winmanifest"

	//"gorgonia.org/gorgonia/examples/mnist"
	"github.com/c-bata/goptuna"
	"github.com/c-bata/goptuna/successivehalving"
	"github.com/c-bata/goptuna/tpe"
	"golang.org/x/sync/errgroup"

	"github.com/dathoangnd/gonet"
)

type UIWindow struct {
	TrainData string
	TestData  string
	Model     ModelConfig
}

type ModelConfig struct {
	ModelID   int
	Name      string
	NumTrials int
	NumNodes  int
	NeuralNet
}

type NeuralNet struct {
	Study           *goptuna.Study
	ErrGroup        *errgroup.Group
	NumHiddenLayers int
	NumEpochs       int
	LearningRate    float64
	Momentum        float64
}

func setupUI() {
	mainwin = ui.NewWindow("Distributed Neural Network", 1200, 900, true)

	mainwin.OnClosing(func(*ui.Window) bool {
		ui.Quit()
		return true
	})
	mainwin.SetMargined(true)
	mainwin.SetChild(makeToolbar2())

	mainwin.Show()
}

func makeToolbar2() ui.Control {
	vbox := ui.NewVerticalBox()
	vbox.SetPadded(true)

	msggrid := ui.NewGrid()
	msggrid.SetPadded(true)
	vbox.Append(msggrid, false)

	pbar = ui.NewProgressBar()
	pbar.SetValue(0)
	vbox.Append(pbar, false)

	grid := ui.NewGrid()
	grid.SetPadded(true)
	vbox.Append(grid, false)

	params := ui.NewGrid()
	params.SetPadded(true)
	vbox.Append(params, false)
	/*for i := 0; i < 6; i++ {
		resList[i] = ui.NewLabel("")
		params.Append(paramList[i],
			0, i, 1, 1,
			false, ui.AlignFill, false, ui.AlignFill)
	}*/
	/*params.Append(ui.NewLabel("Runtime"),
		0, 0, 1, 1,
		false, ui.AlignFill, false, ui.AlignFill)
	params.Append(ui.NewLabel("Epochs"),
		0, 1, 1, 1,
		false, ui.AlignFill, false, ui.AlignFill)
	params.Append(ui.NewLabel("Hidden Layers"),
		0, 2, 1, 1,
		false, ui.AlignFill, false, ui.AlignFill)
	params.Append(ui.NewLabel("Learning Rate"),
		0, 3, 1, 1,
		false, ui.AlignFill, false, ui.AlignFill)
	params.Append(ui.NewLabel("Momentum"),
		0, 4, 1, 1,
		false, ui.AlignFill, false, ui.AlignFill)*/
	//vbox.Append(params, false)

	results = ui.NewGrid()
	results.SetPadded(true)
	vbox.Append(results, false)

	button := ui.NewButton("Import Config")
	button.OnClicked(func(*ui.Button) {
		filename := ui.OpenFile(mainwin)
		if filename != "" {
			file, _ := ioutil.ReadFile(filename)
			temp := UIWindow{}
			_ = json.Unmarshal([]byte(file), &temp)
			windowData = temp
			vbox.Delete(3)
			vbox.Delete(2)
			grid = generateFromState()
			vbox.Append(grid, false)
			results = ui.NewGrid()
			results.SetPadded(true)
			vbox.Append(results, false)
			for i := 0; i < windowData.Model.NumTrials; i++ {
				s := fmt.Sprintf("Trial %d - ", i)
				results.Append(ui.NewLabel(s),
					0, i, 1, 1,
					false, ui.AlignFill, false, ui.AlignFill)
			}
		}
	})
	msggrid.Append(button,
		0, 0, 1, 1,
		false, ui.AlignFill, false, ui.AlignFill)

	/*button = ui.NewButton("Add Model")
	button.OnClicked(func(*ui.Button) {
		if modelCount < 5 {
			var m ModelConfig //configuration
			m.Name = ""
			m.ModelID = 0
			model := makeModelParam(m)
			grid.Append(model,
				0, 2, 2, 1,
				true, ui.AlignFill, false, ui.AlignFill)
			modelCount++
		}
	})
	msggrid.Append(button,
		1, 0, 1, 1,
		false, ui.AlignFill, false, ui.AlignFill)*/
	var m ModelConfig
	m.Name = ""
	m.ModelID = 0
	model := makeModelParam(m)
	grid.Append(model,
		0, 3, 2, 1,
		true, ui.AlignFill, false, ui.AlignFill)

	button = ui.NewButton("Save Config")
	button.OnClicked(func(*ui.Button) {
		filename := ui.SaveFile(mainwin)
		file, _ := json.MarshalIndent(windowData, "", " ")
		_ = ioutil.WriteFile(filename, file, 0644)
	})
	msggrid.Append(button,
		2, 0, 1, 1,
		false, ui.AlignFill, false, ui.AlignFill)

	button = ui.NewButton("Clear All")
	button.OnClicked(func(*ui.Button) {
		temp := UIWindow{}
		windowData = temp
		pbar.SetValue(0)
		vbox.Delete(3)
		vbox.Delete(2)
		grid = generateFromState()
		vbox.Append(grid, false)
		results = ui.NewGrid()
		results.SetPadded(true)
		vbox.Append(results, false)
	})
	msggrid.Append(button,
		3, 0, 1, 1,
		false, ui.AlignFill, false, ui.AlignFill)

	button = ui.NewButton("Run")
	button.OnClicked(func(*ui.Button) {
		paramList = make([]*ui.Label, 6)
		for i := 0; i < 6; i++ {
			paramList[i] = ui.NewLabel("")
			params.Append(paramList[i],
				0, i, 1, 1,
				false, ui.AlignFill, false, ui.AlignFill)
		}
		totTrials := windowData.Model.NumTrials * windowData.Model.NumNodes
		resList = make([]*ui.Label, totTrials)
		for i := 0; i < totTrials; i++ {
			resList[i] = ui.NewLabel("")
			results.Append(resList[i],
				0, i+6, 1, 1,
				false, ui.AlignFill, false, ui.AlignFill)
		}
		go launchServers(windowData.Model.NumNodes, windowData.Model.ErrGroup, windowData.Model.Study)
		pbar.SetValue(-1)
	})
	msggrid.Append(button,
		4, 0, 1, 1,
		false, ui.AlignFill, false, ui.AlignFill)

	button = ui.NewButton("Training Data")
	entry := ui.NewEntry()
	entry.SetReadOnly(true)
	button.OnClicked(func(*ui.Button) {
		filename := ui.OpenFile(mainwin)
		if filename == "" {
			filename = "No File Selected"
		}
		x := parseCSV(filename)
		if len(x) != 0 {
			entry.SetText(filename)
			windowData.TrainData = filename
		} else {
			ui.MsgBoxError(mainwin,
				"Error",
				"CSV not provided or improperly formatted.")
			entry.SetText("No File Selected")
		}

	})
	grid.Append(button,
		0, 0, 1, 1,
		false, ui.AlignFill, false, ui.AlignFill)
	grid.Append(entry,
		1, 0, 1, 1,
		true, ui.AlignFill, false, ui.AlignFill)

	button1 := ui.NewButton("Test Data")
	entry1 := ui.NewEntry()
	entry1.SetReadOnly(true)
	button1.OnClicked(func(*ui.Button) {
		filename := ui.OpenFile(mainwin)
		if filename == "" {
			filename = "No File Selected"
		}
		x := parseCSV(filename)
		if len(x) != 0 {
			entry1.SetText(filename)
			windowData.TestData = filename
		} else {
			ui.MsgBoxError(mainwin,
				"Error",
				"CSV not provided or improperly formatted.")
			entry1.SetText("No File Selected")
		}
	})

	numTrials := ui.NewSpinbox(0, 10000)
	numTrials.SetValue(windowData.Model.NumTrials)
	numTrials.OnChanged(func(*ui.Spinbox) {
		windowData.Model.NumTrials = numTrials.Value()
	})
	form := ui.NewForm()
	form.SetPadded(true)
	form.Append("# Trials per Node", numTrials, false)

	numNodes := ui.NewSpinbox(0, 10000)
	numNodes.SetValue(windowData.Model.NumTrials)
	numNodes.OnChanged(func(*ui.Spinbox) {
		windowData.Model.NumNodes = numNodes.Value()
	})
	form1 := ui.NewForm()
	form1.SetPadded(true)
	form1.Append("# Nodes", numNodes, false)

	grid.Append(button1,
		0, 1, 1, 1,
		false, ui.AlignFill, false, ui.AlignFill)
	grid.Append(entry1,
		1, 1, 1, 1,
		true, ui.AlignFill, false, ui.AlignFill)
	grid.Append(form,
		0, 2, 1, 1,
		false, ui.AlignFill, false, ui.AlignFill)
	grid.Append(form1,
		1, 2, 1, 1,
		true, ui.AlignFill, false, ui.AlignFill)

	return vbox
}

func generateFromState() *ui.Grid {
	grid := ui.NewGrid()
	grid.SetPadded(true)

	button := ui.NewButton("Training Data")
	entry := ui.NewEntry()
	entry.SetReadOnly(true)
	entry.SetText(windowData.TrainData)
	button.OnClicked(func(*ui.Button) {
		filename := ui.OpenFile(mainwin)
		if filename == "" {
			filename = "No File Selected"
		}
		entry.SetText(filename)
		windowData.TrainData = filename
	})
	grid.Append(button,
		0, 0, 1, 1,
		false, ui.AlignFill, false, ui.AlignFill)
	grid.Append(entry,
		1, 0, 1, 1,
		true, ui.AlignFill, false, ui.AlignFill)

	button1 := ui.NewButton("Test Data")
	entry1 := ui.NewEntry()
	entry1.SetReadOnly(true)
	entry1.SetText(windowData.TestData)
	button1.OnClicked(func(*ui.Button) {
		filename := ui.OpenFile(mainwin)
		if filename == "" {
			filename = "No File Selected"
		}
		entry1.SetText(filename)
		windowData.TestData = filename
	})
	grid.Append(button1,
		0, 1, 1, 1,
		false, ui.AlignFill, false, ui.AlignFill)
	grid.Append(entry1,
		1, 1, 1, 1,
		true, ui.AlignFill, false, ui.AlignFill)
	//modelCount = windowData.ModelCount
	/*for i := 0; i < windowData.ModelCount; i++ {
		m := makeModelParam(windowData.Models[i])
		grid.Append(m,
			0, i+2, 2, 1,
			true, ui.AlignFill, false, ui.AlignFill)
	}*/
	return grid
}

var mainwin *ui.Window
var modelCount int = 0
var windowData UIWindow
var results *ui.Grid
var resList []*ui.Label = make([]*ui.Label, 35)
var paramList []*ui.Label = make([]*ui.Label, 5)
var pbar *ui.ProgressBar
var trialCount Counter = Counter{count: 0}

type Counter struct {
	mu    sync.Mutex
	count int
}

func makeModelParam(m ModelConfig) ui.Control {
	fmt.Println("in makeModelParam")
	hbox := ui.NewHorizontalBox()
	hbox.SetPadded(true)

	grid := ui.NewGrid()
	grid.SetPadded(true)

	hbox.Append(grid, false)

	form := ui.NewForm()
	form.SetPadded(true)
	grid.Append(form,
		0, 0, 1, 1,
		false, ui.AlignFill, false, ui.AlignFill)

	initial := 0
	if m.Name == "Bayesian" {
		initial = 1
	}
	alignment := ui.NewCombobox()

	alignment.Append("Random")
	alignment.Append("Bayesian")
	alignment.SetSelected(initial)
	alignment.OnSelected(func(*ui.Combobox) {
		s := alignment.Selected()
		if s == 0 {
			windowData.Model.Name = "Random"

			pruner, _ := successivehalving.NewPruner(successivehalving.OptionReductionFactor(3))
			study, _ := goptuna.CreateStudy(
				"gorgonia-mnist",
				goptuna.StudyOptionSampler(goptuna.NewRandomSampler()),
				goptuna.StudyOptionPruner(pruner),
				goptuna.StudyOptionDirection(goptuna.StudyDirectionMaximize),
			)
			eg, ctx := errgroup.WithContext(context.Background())
			study.WithContext(ctx)

			windowData.Model.Study = study
			windowData.Model.ErrGroup = eg
		} else if s == 1 {
			windowData.Model.Name = "Bayesian"

			pruner, _ := successivehalving.NewPruner(successivehalving.OptionReductionFactor(3))
			study, _ := goptuna.CreateStudy(
				"gorgonia-mnist",
				goptuna.StudyOptionSampler(tpe.NewSampler()),
				goptuna.StudyOptionPruner(pruner),
				goptuna.StudyOptionDirection(goptuna.StudyDirectionMaximize),
			)

			eg, ctx := errgroup.WithContext(context.Background())
			study.WithContext(ctx)

			windowData.Model.Study = study
			windowData.Model.ErrGroup = eg
		}
	})

	form.Append("Model", alignment, false)
	return hbox
}

func parseCSV(path string) [][][]float64 {
	data := make([][][]float64, 0)
	x := make([][][]float64, 0)

	// Open the file
	csvfile, err := os.Open(path)
	if err != nil {
		// log.Fatalln("Couldn't open the csv file", err)
		fmt.Println("Could not open csv:", path)
		return x
	}

	r := csv.NewReader(csvfile)

	index := 0

	// Parse Data
	for {
		// Read each record from csv
		record, err := r.Read()
		if index != 0 && len(record) > 1 {
			floatarr := make([]float64, len(record)-1)
			expected := make([]float64, 10)
			for i := 0; i < len(record); i++ {
				if s, err := strconv.ParseFloat(record[i], 64); err == nil {
					if i == 0 {
						expected[int(s)] = 1
					} else {
						floatarr[i-1] = s
					}
				}
			}
			if len(floatarr) == 0 {
				break
			}
			oneEntry := [][]float64{floatarr, expected}
			data = append(data, oneEntry)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println("CSV could not be read")
			return x
			// log.Fatal(err)
		}
		index++
	}
	return data
}

// Runs a basic neural network
func objective(trial goptuna.Trial) (float64, error) {
	//train := parseCSV("data/mnist_train_new.csv")
	//test := parseCSV("data/mnist_test_new.csv")
	test := parseCSV(windowData.TestData)
	train := parseCSV(windowData.TrainData)

	NumHiddenLayers, _ := trial.SuggestInt("HiddenLayers", 2, 10)
	NumEpochs, _ := trial.SuggestInt("Epochs", 1, 10) //10,50
	LearningRate, _ := trial.SuggestFloat("LearningRate", 1e-5, 1e-1)
	Momentum, _ := trial.SuggestFloat("Momentum", 1e-5, 1e-1)
	hidden := make([]int, NumHiddenLayers)
	for j := 0; j < NumHiddenLayers; j++ {
		hidden[j] = 32
	}
	nn := gonet.New(len(train[0][0]), hidden, len(train[0][1]), true)

	nn.Train(train, NumEpochs, LearningRate, Momentum, false)

	totalcorrect := 0.0
	for i := 0; i < len(test); i++ {
		if MinMax(test[i][1]) == MinMax(nn.Predict(test[i][0])) {
			totalcorrect += 1.0
		}
	}

	ui.QueueMain(func() {
		trialCount.mu.Lock()
		acc := fmt.Sprintf("%d Accuracy: %.2f%%", trialCount.count, totalcorrect/float64(len(test))*100.0)
		resList[trialCount.count].SetText(acc)
		trialCount.count++
		trialCount.mu.Unlock()
		//resList = append(resList, ui.NewLabel(acc))
	})
	return float64(totalcorrect / float64(len(test)) * 100.0), nil
}

/*func objective(trial goptuna.Trial) (float64, error) {
	////////////////////////////////////////////////////////////////////////////
	trainX, trainY, err := mnist.Load("train", "./testdata", g.Float32)
	require.NoError(err)

	exampleSize := trainX.Shape()[0]

	testX, testY, err := mnist.Load("test", "./testdata", g.Float32)
	require.NoError(err)

	batchSize, _ := trial.SuggestInt("batchSize", 4, 128)

	batches := exampleSize / batchSize

	xi := mo.NewInput("x", []int{1, 1, 28, 28})

	yi := mo.NewInput("y", []int{1, 10})

	model, err := mo.NewSequential("mnist")
	require.NoError(err)

	model.AddLayers(
		layer.Conv2D{Input: 1, Output: 32, Width: 3, Height: 3},
		layer.MaxPooling2D{},
		layer.Conv2D{Input: 32, Output: 64, Width: 3, Height: 3},
		layer.MaxPooling2D{},
		layer.Conv2D{Input: 64, Output: 128, Width: 3, Height: 3},
		layer.MaxPooling2D{},
		layer.Flatten{},
		layer.FC{Input: 128 * 3 * 3, Output: 100},
		layer.FC{Input: 100, Output: 10, Activation: layer.Softmax},
	)

	var optimizer g.Solver
	solverName, _ := trial.SuggestCategorical("solver", []string{"Adam", "RMS", "Vanilla"})
	if solverName == "Adam" {
		optimizer = g.NewAdamSolver(g.WithBatchSize(float64(batchSize)))
	} else if solverName == "RMS" {
		optimizer = g.NewRMSPropSolver(g.WithBatchSize(float64(batchSize)))
	} else if solverName == "Vanilla" {
		learnRate, _ := trial.SuggestFloat("learnRate", 1e-5, 1e-1)
		optimizer = g.NewVanillaSolver(g.WithLearnRate(learnRate))
	}
	numEpochs, _ := trial.SuggestInt("numEpochs", 1, 2)
	err = model.Compile(xi, yi,
		mo.WithOptimizer(optimizer),
		mo.WithLoss(mo.CrossEntropy),
		mo.WithBatchSize(batchSize),
	)
	require.NoError(err)

	timer := time.Now()
	for epoch := 0; epoch < numEpochs; epoch++ {
		for batch := 0; batch < batches; batch++ {
			start := batch * batchSize
			end := start + batchSize
			if start >= exampleSize {
				break
			}
			if end > exampleSize {
				end = exampleSize
			}

			xi, err := trainX.Slice(dense.MakeRangedSlice(start, end))
			require.NoError(err)
			xi.Reshape(batchSize, 1, 28, 28)

			yi, err := trainY.Slice(dense.MakeRangedSlice(start, end))
			require.NoError(err)
			yi.Reshape(batchSize, 10)

			err = model.FitBatch(xi, yi)
			require.NoError(err)
			model.Tracker.LogStep(epoch, batch)
		}
		accuracy, loss, err := evaluate(testX.(*tensor.Dense), testY.(*tensor.Dense), model, batchSize)
		require.NoError(err)
		log.Infof("completed train epoch %v with accuracy %v and loss %v", epoch, accuracy, loss)
	}
	runtime := fmt.Sprintf("Runtime: %.5f s, ", time.Since(timer).Seconds())
	acc, _, err := evaluate(testX.(*tensor.Dense), testY.(*tensor.Dense), model, batchSize)
	s := fmt.Sprintf("Results - ")
	//s := fmt.Sprintf("%s Results - ", generateLabel(m.ModelID))
	accu := fmt.Sprintf("Accuracy: %v", acc)
	s = s + runtime + accu

	fmt.Println(s)
	return float64(acc), nil
}*/

/*func evaluate(x, y *tensor.Dense, model *mo.Sequential, batchSize int) (acc, loss float32, err error) {
	exampleSize := x.Shape()[0]
	batches := exampleSize / batchSize

	accuracies := []float32{}
	for batch := 0; batch < batches; batch++ {
		start := batch * batchSize
		end := start + batchSize
		if start >= exampleSize {
			break
		}
		if end > exampleSize {
			end = exampleSize
		}

		xi, err := x.Slice(dense.MakeRangedSlice(start, end))
		require.NoError(err)
		xi.Reshape(batchSize, 1, 28, 28)

		yi, err := y.Slice(dense.MakeRangedSlice(start, end))
		require.NoError(err)
		yi.Reshape(batchSize, 10)

		yHat, err := model.PredictBatch(xi)
		require.NoError(err)

		acc, err := accuracy(yHat.(*tensor.Dense), yi.(*tensor.Dense), model)
		require.NoError(err)
		accuracies = append(accuracies, acc)
	}
	lossVal, err := model.Tracker.GetValue("mnist_train_batch_loss")
	require.NoError(err)
	loss = float32(lossVal.Scalar())
	acc = num.Mean(accuracies)
	return
}*/

/*func accuracy(yHat, y *tensor.Dense, model mo.Model) (float32, error) {
	yMax, err := y.Argmax(1)
	require.NoError(err)

	yHatMax, err := yHat.Argmax(1)
	require.NoError(err)

	eq, err := tensor.ElEq(yMax, yHatMax, tensor.AsSameType())
	require.NoError(err)
	eqd := eq.(*tensor.Dense)
	len := eqd.Len()

	numTrue, err := eqd.Sum()
	if err != nil {
		return 0, err
	}

	return float32(numTrue.Data().(int)) / float32(len), nil
}*/

// Finds the index of max value in an array
func MinMax(array []float64) int {
	index := 0
	max := 0.0
	for i, value := range array {
		if max < value {
			index = i
			max = value
		}
	}
	return index
}

//!!!MAIN!!!
func main() {
	/*fmt.Println("main")
	pruner, _ := successivehalving.NewPruner(successivehalving.OptionReductionFactor(3))
	study, _ := goptuna.CreateStudy(
		"gorgonia-mnist",
		//goptuna.StudyOptionSampler(goptuna.NewRandomSampler()),
		goptuna.StudyOptionSampler(tpe.NewSampler()),
		goptuna.StudyOptionPruner(pruner),
		goptuna.StudyOptionDirection(goptuna.StudyDirectionMaximize),
	)

	eg, ctx := errgroup.WithContext(context.Background())
	study.WithContext(ctx)

	fmt.Println("before launch servers")
	//finished := make(chan bool)
	start := time.Now()
	launchServers(10, eg, study)
	fmt.Printf("Runtime: %.5f s, \n", time.Since(start).Seconds())
	v, _ := study.GetBestValue()
	params, _ := study.GetBestParams()
	fmt.Printf("Best evaluation=%f\n", v)
	/--fmt.Printf("Solver: %s\n", params["solver"].(string))
	if params["solver"].(string) == "Vanilla" {
		fmt.Printf("Learning rate (vanilla): %f\n", params["vanilla_learning_rate"].(float64))
	}*/
	//fmt.Printf("Epochs: %d\n", params["Epochs"].(int))
	//fmt.Printf("Hidden Layers: %d\n", params["HiddenLayers"].(int))
	//fmt.Printf("Learning Rate: %f\n", params["LearningRate"].(float64))
	//fmt.Printf("Momentum: %f\n", params["Momentum"].(float64))
	ui.Main(setupUI)
}

// ------------------------------ PAXOS ------------------------------------
//Replicating processes
// number of workers
var numWorkers int

// struct to organize data into the master function
type MasterData struct {
	id              int
	numWorkers      int
	request         []chan ModelConfig
	commands        []chan string
	dataOut         []chan [][][]float64
	models          []ModelConfig
	replies         []chan string
	corpses         chan []bool
	working         []string
	finished        []string
	toShadowMasters []chan string
	log             []string
	hb1             []chan [][]int64
	hb2             []chan [][]int64
	//test            []chan [][][]float64
	//training        []chan [][][]float64
}

// Launches nodes and creates MasterData structure
func launchServers(numW int, eg *errgroup.Group, study *goptuna.Study) {
	//func launchServers(numW int) {
	start := time.Now()
	fmt.Println("Launching")
	numWorkers = numW

	var mrData MasterData
	mrData.numWorkers = numWorkers
	mrData.request = make([]chan ModelConfig, mrData.numWorkers) // worker <- master : for task assignment
	mrData.commands = make([]chan string, mrData.numWorkers)     // master -> worker : master sends commands
	mrData.replies = make([]chan string, mrData.numWorkers)      // master <- worker : worker reply for task completion
	mrData.corpses = make(chan []bool, mrData.numWorkers)        // master <- heartbeat : workers that have died
	mrData.working = make([]string, mrData.numWorkers)           // which tasks assigned to which workers

	var hb1 = make([]chan [][]int64, mrData.numWorkers+3) // heartbeat channels to neighbors for read
	var hb2 = make([]chan [][]int64, mrData.numWorkers+3) // heartbeat channels to neighbors for write
	killMaster := make(chan string, 10)                   // channel to kill Master to verify replication and recovery from log

	// initialize heartbeat tables
	for i := 0; i < mrData.numWorkers+3; i++ {
		hb1[i] = make(chan [][]int64, 1024)
		hb2[i] = make(chan [][]int64, 1024)
	}
	mrData.hb1 = hb1
	mrData.hb2 = hb2

	// initialize worker channels
	for k := 0; k < mrData.numWorkers; k++ {
		mrData.request[k] = make(chan ModelConfig)
		mrData.replies[k] = make(chan string)
		mrData.working[k] = ""
		mrData.commands[k] = make(chan string)
	}

	// initialize shadowMasters
	numShadowMasters := 2

	// channel shadowMaster <- master : replication
	mrData.toShadowMasters = make([]chan string, numShadowMasters)
	for j := 0; j < numShadowMasters; j++ {
		mrData.toShadowMasters[j] = make(chan string, 10)
	}

	// start nodes
	masterlog := make([]string, 0)
	var endrun = make(chan string)
	go master(eg, study, mrData, hb1, hb2, masterlog, killMaster, endrun)

	go shadowMaster(eg, study, mrData.toShadowMasters[0], hb1, hb2, mrData.numWorkers, mrData.numWorkers+1, mrData, killMaster)

	go shadowMaster(eg, study, mrData.toShadowMasters[1], hb1, hb2, mrData.numWorkers, mrData.numWorkers+2, mrData, killMaster)

	for {
		select {
		case reply := <-endrun:
			if reply == "end" {
				fmt.Println("end launchservers")
				v, _ := study.GetBestValue()
				params, _ := study.GetBestParams()
				bestEval := fmt.Sprintf("Best evaluation=%f\n", v)
				runtime := fmt.Sprintf("Runtime: %.5f s \n", time.Since(start).Seconds())
				epochs := fmt.Sprintf("Epochs: %d\n", params["Epochs"].(int))
				layers := fmt.Sprintf("Hidden Layers: %d\n", params["HiddenLayers"].(int))
				learn := fmt.Sprintf("Learning Rate: %f\n", params["LearningRate"].(float64))
				momentum := fmt.Sprintf("Momentum: %f\n", params["Momentum"].(float64))
				/*paramList[0] = ui.NewLabel(epochs)
				paramList[1] = ui.NewLabel(layers)
				paramList[2] = ui.NewLabel(learn)
				paramList[3] = ui.NewLabel(momentum)*/
				ui.QueueMain(func() {
					paramList[0].SetText(runtime)
					paramList[1].SetText(epochs)
					paramList[2].SetText(layers)
					paramList[3].SetText(learn)
					paramList[4].SetText(momentum)
					paramList[5].SetText(bestEval)
				})

				/*fmt.Printf("Solver: %s\n", params["solver"].(string))
				if params["solver"].(string) == "Vanilla" {
					fmt.Printf("Learning rate (vanilla): %f\n", params["vanilla_learning_rate"].(float64))
				}*/
				return
			}
		}
	}

	//add opt params to gui
	//fmt.Printf("Runtime: %.5f s, \n", time.Since(start).Seconds())
	//fmt.Printf("Epochs: %d\n", params["Epochs"].(int))
	//fmt.Printf("Hidden Layers: %d\n", params["HiddenLayers"].(int))
	//fmt.Printf("Learning Rate: %f\n", params["LearningRate"].(float64))
	//fmt.Printf("Momentum: %f\n", params["Momentum"].(float64))

}

// Master node
func master(eg *errgroup.Group, study *goptuna.Study, mrData MasterData, hb1 []chan [][]int64, hb2 []chan [][]int64, log []string, killMaster chan string, end chan string) {
	//func master(mrData MasterData, hb1 []chan [][]int64, hb2 []chan [][]int64, log []string, killMaster chan string, end chan string) {
	// initialize variables
	currentStep := "step start"
	mrData.log = log
	killHB := make(chan string, numWorkers)
	message := ""

	// Master Recovery: resume step after Master failure
	for i := 0; i < len(log); i++ {
		if len(log[i]) >= 4 && log[i][0:4] == "step" {
			currentStep = log[i]
		}
	}

	// Run master heartbeat
	go masterHeartbeat(eg, study, hb1, hb2, mrData.numWorkers, mrData.corpses, killHB, killMaster, mrData)

	fmt.Println("after master heartbeat")
	for {
		if currentStep == "step start" {
			fmt.Println("step start")
			// If master died partway through launching workers, find the last logged k and start from there
			k := 0
			if len(log) > 0 {
				/*
					len(log)>0 implies master died previously. go through log to findlast launch record.
					Should increment k before going into for loop?
				*/
				last := strings.Split(log[len(log)-1], " ")
				if last[0] == "launch" {
					k, _ = strconv.Atoi(last[2])
				}
			}
			for ; k < mrData.numWorkers; k++ {
				//go worker(mrData.training[k], mrData.test[k], mrData.request[k], mrData.commands[k], mrData.replies[k], hb1, hb2, k)
				//go worker(mrData.training[k], mrData.test[k], mrData.request[k], mrData.commands[k], mrData.replies[k], hb1, hb2, k)
				fmt.Println("launch worker from masterhb")
				go worker(eg, study, mrData.request[k], mrData.commands[k], mrData.replies[k], hb1, hb2, k)
				message = fmt.Sprintf("launch worker %d", k)
				mrData.log = append(mrData.log, currentStep) //appends launch worker step to log
				mrData.toShadowMasters[0] <- message         //sends launch worker message to first Shadow Master channel
				mrData.toShadowMasters[1] <- message         //sends launch worker message to second Shadow Master channel
			}
			currentStep = "step working"
			mrData.log = append(mrData.log, currentStep) //appends load step to log
			mrData.toShadowMasters[0] <- currentStep     //sends load message to first Shadow Master channel
			mrData.toShadowMasters[1] <- currentStep     //sends load message to second Shadow Master channel

		} else if currentStep == "step working" {
			fmt.Println("step working")
			doneDist := make(chan bool)
			mrData = distributeTasks(doneDist, eg, study, mrData)
			fmt.Println("after dt")
			currentStep = "step cleanup"
			mrData.log = append(mrData.log, currentStep) //appends master distributeTasks step to log
			mrData.toShadowMasters[0] <- currentStep     //sends distributeTasks message to first Shadow Master channel
			mrData.toShadowMasters[1] <- currentStep     //sends distributeTasks message to second Shadow Master channel

		} else if currentStep == "step cleanup" {
			// cleanup workers who should now be done with all tasks
			fmt.Println("step cleanup")
			currentStep = "step end"
			mrData.log = append(mrData.log, currentStep) //appends end message to log
			mrData.toShadowMasters[0] <- currentStep     //sends end message to first Shadow Master channel
			mrData.toShadowMasters[1] <- currentStep     //sends end message to second Shadow Master channel

			killHB <- "die"
			for i := 0; i < mrData.numWorkers; i++ {
				mrData.commands[i] <- "end"
			}
			end <- "end"
			fmt.Println("Running master is done.")
			break
		}
	}
}

func distributeTasks(doneDist chan bool, eg *errgroup.Group, study *goptuna.Study, mrData MasterData) MasterData {
	//func distributeTasks(doneDist chan bool, mrData MasterData) MasterData {
	count := 0
	loop := true

	fmt.Println("Distributing Tasks Started...")
	mrData.finished = make([]string, mrData.numWorkers) // which models have completed
	//mrData.finished = make([]string, len(mrData.models)) // which models have completed
	for j := 0; j < len(mrData.models); j++ {
		mrData.finished[j] = "not started"
	}
	for i := 0; i < mrData.numWorkers; i++ {
		mrData.commands[i] <- "m"
		mrData.working[i] = "running"
	}
	for loop {
		for i := 0; i < mrData.numWorkers; i++ {
			// checks for available workers
			if mrData.working[i] == "" {
			}
			// checks for replies and dead workers
			select {
			case message := <-mrData.replies[i]:
				fmt.Println(message)
				//replied := strings.Split(message, "_")
				//workerID, _ := strconv.Atoi(replied[1])
				mrData.working[i] = ""
				mrData.finished[i] = "finished"
				fmt.Println("new count")
				count++
			case coffins := <-mrData.corpses:
				//fmt.Println("corpses")
				for j := 0; j < mrData.numWorkers; j++ {
					if coffins[j] {
						mrData.hb1[j] = make(chan [][]int64, numWorkers+3)
						mrData.hb2[j] = make(chan [][]int64, numWorkers+3)
						mrData.request[j] = make(chan ModelConfig)
						mrData.replies[j] = make(chan string)
						go worker(eg, study, mrData.request[j], mrData.commands[j], mrData.replies[j], mrData.hb1, mrData.hb2, j)
						//go worker(mrData.training[j], mrData.test[j], mrData.request[j], mrData.commands[j], mrData.replies[j], mrData.hb1, mrData.hb2, j)
						tempModelID, _ := strconv.Atoi(mrData.working[j])
						mrData.finished[tempModelID] = "not started"
						mrData.working[j] = ""
						coffins[j] = false
					}
				}
			default:
			}
		}
		// checks that all models have completed
		if count >= mrData.numWorkers {
			//if count >= len(mrData.models) {
			fmt.Println("all models have completed")
			check := true

			for a := 0; a < mrData.numWorkers; a++ {
				if mrData.working[a] != "" {
					check = false
					break
				}
			}
			if check {
				loop = false
			}
		}
	}
	fmt.Println("\nDistributing Finished")
	//doneDist <- true
	return mrData
}

//func worker(train chan [][][]float64, test chan [][][]float64, frommaster chan ModelConfig, commands chan string, reply chan string, hb1 []chan [][]int64, hb2 []chan [][]int64, k int) {
//func worker(train chan []tensor.Tensor, test chan []tensor.Tensor, frommaster chan ModelConfig, commands chan string, reply chan string, hb1 []chan [][]int64, hb2 []chan [][]int64, k int) {
func worker(eg *errgroup.Group, study *goptuna.Study, frommaster chan ModelConfig, commands chan string, reply chan string, hb1 []chan [][]int64, hb2 []chan [][]int64, k int) {
	//func worker(frommaster chan ModelConfig, commands chan string, reply chan string, hb1 []chan [][]int64, hb2 []chan [][]int64, k int) {
	//fmt.Println("worker")
	//var eg *errgroup.Group = windowData.Model.ErrGroup
	//var study *goptuna.Study = windowData.Model.Study
	killHB := make(chan string, 1)
	go heartbeat(hb1, hb2, k, killHB)
	for {
		select {
		case task := <-commands:
			tasks := strings.Split(task, "_")
			if tasks[0] == "end" {
				fmt.Println("worker end")
				killHB <- "end"
				return
			}
			if tasks[0] == "m" {
				/*eg.Go(func() error {
					return study.Optimize(objective, 10) //20
				})*/
				study.Optimize(objective, windowData.Model.NumTrials) //20
				/*if err := eg.Wait(); err != nil {
					log.Fatal("Optimize error", err)
				}*/
				reply <- strconv.Itoa(k) + "_" + "empty"
			}
		default:
		}
	}
}

// Helper function to find max
func max(x, y int64) int64 {
	if x > y {
		return x
	}
	return y
}

func printModelParam(model ModelConfig) {
	fmt.Println("ID", model.ModelID, ", epochs", model.NumEpochs, ", learning rate", model.LearningRate,
		", momentum", model.Momentum, ", hidden layers", model.NumHiddenLayers)
}

// Update heartbeat tables for Master, 2 ShadowMasters and 7 workers
func updateTable(index int, hbtable [][]int64, counter int, hb1 []chan [][]int64, hb2 []chan [][]int64) [][]int64 {
	//index refers to 'k' of worker
	//next and prev are neighboring nodes
	next := index + 1
	prev := index - 1
	//wraparound
	if prev < 0 {
		prev = numWorkers + 2
	}
	if next > numWorkers+2 {
		next = 0
	}

	temp := make([][]int64, numWorkers+3)
	neighbor1 := make([][]int64, numWorkers+3)
	neighbor2 := make([][]int64, numWorkers+3)

	//for each worker + shadowmasters + master
	//copy current node's hbtable to neighbor1/2 and temp
	for i := 0; i < numWorkers+3; i++ {
		neighbor1[i] = make([]int64, 2)
		neighbor1[i][0] = hbtable[i][0]
		neighbor1[i][1] = hbtable[i][1]
		neighbor2[i] = make([]int64, 2)
		neighbor2[i][0] = hbtable[i][0]
		neighbor2[i][1] = hbtable[i][1]
		temp[i] = make([]int64, 2)
		temp[i][0] = hbtable[i][0]
		temp[i][1] = hbtable[i][1]
	}
	loop2 := true
	//unsure what this is for. i think counter is always int so always true
	//hb1 is read, hb2 is write
	if counter%1 == 0 {
		for loop2 {
			select {
			case neighbor1_1 := <-hb1[next]:
				neighbor1 = neighbor1_1
			default:
				loop2 = false
			}
		}
		//get latest time from neighbor's hbtable
		for i := 0; i < numWorkers+3; i++ {
			temp[i][0] = max(neighbor1[i][0], hbtable[i][0])
			temp[i][1] = max(neighbor1[i][1], hbtable[i][1])
		}
		loop2 = true

		for loop2 {
			select {
			case neighbor2_1 := <-hb2[prev]:
				neighbor2 = neighbor2_1
			default:
				loop2 = false
			}
		}
		for i := 0; i < numWorkers+3; i++ {
			temp[i][0] = max(neighbor2[i][0], hbtable[i][0])
			temp[i][1] = max(neighbor2[i][1], hbtable[i][1])
		}
	}
	now := time.Now().Unix() // current local time
	temp[index][0] = hbtable[index][0] + 1
	temp[index][1] = now
	// send table
	hb1[index] <- temp
	hb2[index] <- temp
	return temp
}

// Heartbeat function for all workers
func heartbeat(hb1 []chan [][]int64, hb2 []chan [][]int64, k int, endHB chan string) {
	now := time.Now().Unix() // current local time
	counter := 0
	hbtable := make([][]int64, numWorkers+3)
	// initialize hbtable
	for i := 0; i < numWorkers+3; i++ {
		hbtable[i] = make([]int64, 2)
		hbtable[i][0] = 0
		hbtable[i][1] = now
	}

	for {
		time.Sleep(100 * time.Millisecond)
		hbtable = updateTable(k, hbtable, counter, hb1, hb2)
		counter++
		select {
		case reply := <-endHB:
			if reply == "end" {
				fmt.Println("ending worker hb")
				return
			}
		default:
		}
	}
}

// Heartbeat function for Master
func masterHeartbeat(eg *errgroup.Group, study *goptuna.Study, hb1 []chan [][]int64, hb2 []chan [][]int64, k int, corpses chan []bool, kill chan string, killMaster chan string, mrData MasterData) {
	//func masterHeartbeat(hb1 []chan [][]int64, hb2 []chan [][]int64, k int, corpses chan []bool, kill chan string, killMaster chan string, mrData MasterData) {
	now := time.Now().Unix() // current local time
	counter := 0
	currentTable := make([][]int64, numWorkers+3)
	previousTable := make([][]int64, numWorkers+3)

	fmt.Println("master heartbeat started")

	// initialize hbtable
	for i := 0; i < numWorkers+3; i++ {
		currentTable[i] = make([]int64, 2)
		previousTable[i] = make([]int64, 2)
		currentTable[i][0] = 0
		previousTable[i][0] = 0
		currentTable[i][1] = now
		previousTable[i][1] = now
	}
	deadWorkers := make([]bool, numWorkers+3)
	for i := 0; i < numWorkers+3; i++ {
		deadWorkers[i] = false
	}
	for {
		//fmt.Print("master start of for")
		select {
		case reply := <-kill:
			if reply == "die" {
				fmt.Println("master heartbeat ended")
				return
			}
		default:
		}
		time.Sleep(100 * time.Millisecond)
		currentTable = updateTable(k, previousTable, counter, hb1, hb2)
		//checks all, including self to see if >15ms since last heartbeat --> dead process
		for i := 0; i < numWorkers+3; i++ {
			if currentTable[k][1]-previousTable[i][1] > 15 { // i is dead
				fmt.Println(currentTable[k][1], previousTable[i][1])
				if i == numWorkers+1 || i == numWorkers+2 { // if shadowmaster dead
					fmt.Println("Shadow master died")
					go shadowMaster(eg, study, mrData.toShadowMasters[i-numWorkers], mrData.hb1, mrData.hb2, mrData.numWorkers, i, mrData, killMaster)
				} else {
					fmt.Println("\n\n-------------------killed worker :", i, "\n")
					deadWorkers[i] = true
				}
			}
		}
		previousTable = currentTable
		corpses <- deadWorkers
		counter++
	}
}

// Shadow Master Node
func shadowMaster(eg *errgroup.Group, study *goptuna.Study, copier chan string, hb1 []chan [][]int64, hb2 []chan [][]int64, masterID int, selfID int, mrData MasterData, kill chan string) {
	//func shadowMaster(copier chan string, hb1 []chan [][]int64, hb2 []chan [][]int64, masterID int, selfID int, mrData MasterData, kill chan string) {
	// replicate logs to two shadowmasters that monitor if the master dies
	fmt.Println("inside shadowMaster")
	logs := make([]string, 0)
	killHB := make(chan string, 3)
	var isMasterDead = make(chan bool, 1)
	go shadowHeartbeat(hb1, hb2, masterID, isMasterDead, selfID, killHB)
	masterNotDead := true

	for masterNotDead {
		select {
		case copy := <-copier:
			logs = append(logs, copy)
			// check if to die
			currentStep := copy
			if currentStep == "step start" {
				// update logs
				currentStep = "step load"
				mrData.log = append(mrData.log, currentStep)

			} else if currentStep == "step working" {
				// master working
				currentStep = "step cleanup"
				mrData.log = append(mrData.log, currentStep)

			} else if currentStep == "step cleanup" {
				// cleanup
				currentStep = "step end"
				mrData.log = append(mrData.log, currentStep)
				killHB <- "kill"
				return
			}
		case isDead := <-isMasterDead:
			masterNotDead = isDead
		default:
		}
		if !masterNotDead {
			fmt.Println("Shadow master becomes running master.")
			var endrun = make(chan string)
			go master(eg, study, mrData, hb1, hb2, logs, kill, endrun)
			masterNotDead = true
		}
	}
}

// Heartbeat for Shadow Master
func shadowHeartbeat(hb1 []chan [][]int64, hb2 []chan [][]int64, masterID int, isMasterAlive chan bool, selfID int, killHB chan string) string {
	now := time.Now().Unix() // current local time
	counter := 0
	currentTable := make([][]int64, numWorkers+3)
	previousTable := make([][]int64, numWorkers+3)

	// initialize hbtable
	for i := 0; i < numWorkers+3; i++ {
		currentTable[i] = make([]int64, 2)
		previousTable[i] = make([]int64, 2)
		currentTable[i][0] = 0
		previousTable[i][0] = 0
		currentTable[i][1] = now
		previousTable[i][1] = now
	}

	for {
		select {
		case reply := <-killHB:
			fmt.Println("kill shadow heartbeat")
			return reply
		default:
		}
		time.Sleep(100 * time.Millisecond)
		currentTable = updateTable(selfID, previousTable, counter, hb1, hb2)

		if currentTable[selfID][1]-previousTable[masterID][1] > 15 {
			if selfID == masterID+1 {
				fmt.Println("\n----- The Running Master has died -----\n")
				isMasterAlive <- false
				currentTable = updateTable(masterID, currentTable, counter, hb1, hb2)
			}
		}
		previousTable = currentTable
		counter++
	}
}
