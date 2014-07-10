package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/MG-RAST/AWE/lib/conf"
	"github.com/MG-RAST/AWE/lib/logger"
	"io/ioutil"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"
)

//var app_registry_url = "https://raw.githubusercontent.com/wgerlach/SODOKU/master/apps/apps.json"
var MyAppRegistry AppRegistry

type AppCommandMode struct {
	Input           [][]string          `bson:"input" json:"input"`
	Output_array    []string            `bson:"output_array" json:"output_array"`
	Output_map      map[string]string   `bson:"output_map" json:"output_map"`
	Predata         IOmap               `bson:"predata" json:"predata"`
	Cmd             string              `bson:"cmd" json:"cmd"`
	Cmd_interpreter string              `bson:"cmd_interpreter" json:"cmd_interpreter"`
	Cmd_script      []string            `bson:"cmd_script" json:"cmd_script"`
	Variables       []map[string]string `bson:"variables" json:"variables"`
	Dockerimage     string              // just for convenience
}

type AppPackage struct {
	Dockerimage string                                `bson:"dockerimage" json:"dockerimage"`
	Commands    map[string]map[string]*AppCommandMode // package_command, package_mode
}

type AppRegistry map[string]*AppPackage

type AppResource struct {
	Resource       string `bson:"resource" json:"resource"`
	Host           string `bson:"host" json:"host"`
	Node           string `bson:"node" json:"node"`
	Filename       string `bson:"filename" json:"filename"`
	Key            string `bson:"key" json:"key"`
	Value          string `bson:"value" json:"value"`
	Task           string `bson:"task" json:"task"`
	OutputPosition *int   `bson:"position" json:"position"`
	OutputName     string `bson:"name" json:"name"`
}

type AppInputType int

const (
	Ait_undefined AppInputType = iota
	Ait_file
	Ait_string
	Ait_shock
	Ait_task
)

type AppVariable struct {
	Value    string
	Var_type AppInputType
}

type AppVariables map[string]AppVariable

type VariableExpander struct {
	simple_variable_match     *regexp.Regexp
	functional_variable_match *regexp.Regexp
	app_variables             AppVariables
}

func (this_ait AppInputType) HasType(ait AppInputType) bool {
	if this_ait == ait {
		return true
	}
	// file hasType shock = false
	// shock hasType file = true

	if ait == Ait_file {
		if this_ait == Ait_shock {
			return true
		}
		if this_ait == Ait_task {
			return true
		}
	}
	return false
}

func string2apptype(type_string string) (ait AppInputType, err error) {

	if type_string == "file" {
		ait = Ait_file
	} else if type_string == "string" {
		ait = Ait_string
	} else if type_string == "shock" {
		ait = Ait_shock
	} else if type_string == "task" {
		ait = Ait_task
	} else {
		err = errors.New(fmt.Sprintf("could not convert type: %s", type_string))
	}

	return
}

func apptype2string(ait AppInputType) string {
	switch ait {
	case Ait_undefined:
		return "undefined"
	case Ait_file:
		return "file"
	case Ait_string:
		return "string"
	case Ait_shock:
		return "shock"
	case Ait_task:
		return "task"

	}

	return "unknown"

}

// generator function for app registry
func MakeAppRegistry() (new_instance AppRegistry, err error) {

	if conf.APP_REGISTRY_URL == "" {
		err = errors.New("error app registry url empty")
		return
	}

	new_instance = make(AppRegistry)

	//new_instance.packages = make(map[string]*AppPackage)

	for i := 0; i < 3; i++ {
		c := make(chan bool, 1)
		go func() {
			res, err := http.Get(conf.APP_REGISTRY_URL)
			c <- true //we are ending
		}()
		select {
		case <-c:
			//go ahead
		case <-time.After(5): //GET timeout
			return "", errors.New("timeout")
		}
		if err == nil {
			break
		}
		if i == 3 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if err != nil {
		err = errors.New("downloading app registry, error=" + err.Error())

		return
	}
	//logger.Debug(1, fmt.Sprintf("app downloaded: %s", parsed.workunit.Cmd.Name))

	app_registry_json, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	//logger.Debug(1, fmt.Sprintf("app stored in app_registry_json: %s", parsed.workunit.Cmd.Name))

	if err != nil {
		err = errors.New("error reading app registry, error=" + err.Error())
		return
	}
	//logger.Debug(1, fmt.Sprintf("app registry downloaded: %s", app_string))

	//fmt.Printf("%s", app_registry_json)

	// transform json into go struct interface
	//var f map[string]interface{}
	err = json.Unmarshal(app_registry_json, &new_instance)

	if err != nil {
		err = errors.New("error unmarshaling app registry, error=" + err.Error())
		return
	}
	logger.Debug(1, fmt.Sprintf("app registry unmarshalled"))

	return
}

func (appr AppRegistry) Get_cmd_mode_object(app_package_name string, app_command_name string, app_cmd_mode_name string) (app_cmd_mode_object_ref *AppCommandMode, err error) {

	// map app call to registry
	app_package_object_ref, ok := appr[app_package_name]
	if !ok {
		err = errors.New("app_package_name=" + app_package_name + " not found in app registry")
		return
	}

	app_command_object_ref, ok := app_package_object_ref.Commands[app_command_name]
	if !ok {
		err = errors.New("app_command_name=" + app_command_name + " not found in app registry")

		return
	}

	app_cmd_mode_object_ref, ok = app_command_object_ref[app_cmd_mode_name]
	if !ok {
		err = errors.New("app_cmd_mode_name=" + app_cmd_mode_name + " not found in app registry")
		return
	}

	if app_cmd_mode_object_ref.Dockerimage == "" {
		app_cmd_mode_object_ref.Dockerimage = app_package_object_ref.Dockerimage
	}
	//app_cmd_mode_object_ref = &app_cmd_mode_object

	return
}

func (appr AppRegistry) Get_dockerimage(app_package_name string) (dockerimage string, err error) {
	app_package_object_ref, ok := appr[app_package_name]
	if !ok {
		err = errors.New("app_package_name=" + app_package_name + " not found in app registry")
		return
	}

	return app_package_object_ref.Dockerimage, err
}

func (acm AppCommandMode) Get_default_app_variables() (app_variables AppVariables, err error) {
	app_variables = make(AppVariables)

	// *** app input arguments (app definition)
	logger.Debug(1, fmt.Sprintf("Get_default_app_variables: size of acm.Input=%d", len(acm.Input)))
	time.Sleep(15 * time.Millisecond)
	for _, input_arg := range acm.Input {
		logger.Debug(1, fmt.Sprintf("app input arg: %s", strings.Join(input_arg, ", ")))

		// save the defaults if available

		if len(input_arg) >= 2 {
			variable_name := input_arg[1]
			logger.Debug(1, fmt.Sprintf("from app-definition: variable \"%s\"", variable_name))
			app_type, err := string2apptype(input_arg[0])
			if err != nil {
				err = errors.New(fmt.Sprintf("error converting type, error=%s", err.Error()))
				return app_variables, err
			}
			logger.Debug(1, fmt.Sprintf("from app-definition: variable \"%s\" has type %s", variable_name, apptype2string(app_type)))

			if len(input_arg) >= 3 && input_arg[2] != "" {

				logger.Debug(1, fmt.Sprintf("from app-definition: write variable:\"%s\" - default value: \"%s\"", variable_name, input_arg[2]))
				app_variables[variable_name] = AppVariable{Value: input_arg[2], Var_type: app_type}

			} else {
				app_variables[variable_name] = AppVariable{Var_type: app_type}
			}

		}

	}

	return
}

func (acm AppCommandMode) Get_app_variables(app_variables AppVariables) (err error) {
	va := NewVariableExpander(app_variables)
	logger.Debug(1, fmt.Sprintf("---------variable block start"))
	for _, variable_block := range acm.Variables {
		for variable_name, variable_term := range variable_block {

			// evaluate and add to app_variables
			expanded_var, err := va.Expand(variable_term)
			if err != nil {
				return err
			}
			logger.Debug(1, fmt.Sprintf("variable_name: %s -> %s", variable_name, expanded_var))
			variable_obj, ok := app_variables[variable_name]
			if ok {
				variable_obj.Value = expanded_var
			} else {
				app_variables[variable_name] = AppVariable{Var_type: Ait_string, Value: expanded_var}
			}

		}
	}
	logger.Debug(1, fmt.Sprintf("---------variable block end"))
	return
}

// Overview of createIOnodes_forTask
// -------------------------------
// recurse into task dependencies
// get app object
// app input           -> variables
// task input          -> variables
// app/task variables  -> variables
// eval outputs
// create outputs
// create inputs
// extend dependsOn
func (appr AppRegistry) createIOnodes_forTask(job *Job, task *Task, taskid2task map[string]*Task, taskid_processed map[string]bool) (err error) {

	taskid_split := strings.Split(task.Id, "_")
	taskid := taskid_split[1]

	// already processed ?
	_, ok := taskid_processed[taskid]
	if ok {
		return
	}
	taskid_processed[taskid] = true

	// is it an app ?
	if !strings.HasPrefix(task.Cmd.Name, "app:") {
		return
	}

	// recurse into providing
	args_array := task.Cmd.App_args

	for _, argument := range args_array {
		if argument.Resource == "task" {
			providing_taskid := argument.Task
			logger.Debug(1, fmt.Sprintf("recursion from %s into %s", taskid, providing_taskid))
			providing_task := taskid2task[providing_taskid]
			err = appr.createIOnodes_forTask(job, providing_task, taskid2task, taskid_processed)
			if err != nil {
				return
			}
			logger.Debug(1, fmt.Sprintf("back from recursion (%s into %s)", taskid, providing_taskid))
		}
	}

	app_string := strings.TrimPrefix(task.Cmd.Name, "app:")

	app_array := strings.Split(app_string, ".")

	if len(app_array) != 3 {

		err = errors.New("error: app could not be parsed, app=" + app_string)
		return
	}

	app_cmd_mode_object, err := appr.Get_cmd_mode_object(app_array[0], app_array[1], app_array[2])

	if err != nil {
		err = errors.New(fmt.Sprintf("error reading app registry, error=%s", err.Error()))
		return err
	}

	// create app_variables from app-input definition
	logger.Debug(1, fmt.Sprintf("+++ %s +++ create app_variables from app-input definition", task.Id))
	app_variables, err := app_cmd_mode_object.Get_default_app_variables()
	if err != nil {
		return err
	}
	task.AppVariables = app_variables

	// add variables from task input (args_array)
	logger.Debug(1, fmt.Sprintf("+++ %s +++ add variables from task input (args_array)", task.Id))
	err = app_cmd_mode_object.ParseAppInput(app_variables, args_array, nil, nil, taskid2task)
	if err != nil {
		return errors.New(fmt.Sprintf("error parsing input, error=%s", err.Error()))
	}

	// add variables from the app variables defintion
	logger.Debug(1, fmt.Sprintf("+++ %s +++ add variables from the app variables defintion", task.Id))
	app_cmd_mode_object.Get_app_variables(app_variables)

	// create ouputs
	logger.Debug(1, fmt.Sprintf("+++ %s +++ create outputs", task.Id))
	if task.Outputs == nil {
		task.Outputs = make(IOmap)
	}

	task_outputs := task.Outputs

	output_array_copy := make([]string, len(app_cmd_mode_object.Output_array))
	copy(output_array_copy, app_cmd_mode_object.Output_array)

	err = Expand_app_variables(app_variables, output_array_copy)
	if err != nil {
		return
	}

	shockhost := job.ShockHost // TODO do something if this is not defined/empty

	for pos, app_output := range output_array_copy {
		if app_output == "" {
			return errors.New("error: app_output is empty string")
		}
		filename := path.Base(app_output)
		directory := path.Dir(app_output)

		logger.Debug(1, fmt.Sprintf("output: filename: \"%s\", directory: \"%s\", (was: \"%s\")", filename, directory, app_output))
		if directory == "." {
			directory = "" // TODO "." might be ok
		}

		my_io := &IO{Host: shockhost, Directory: directory, AppPosition: pos, DataToken: task.Info.DataToken}
		task_outputs[filename] = my_io

	}

	// TODO output from map

	//for key, value := range mymap {
	//my_io := &IO{Host: shockhost, Directory: directory, AppName: XXX}
	// TODO this could add an output file already listed in the array, that is ok I think
	//}

	// populate with input fields:
	logger.Debug(1, fmt.Sprintf("+++ %s +++ populate with input fields", task.Id))
	err = app_cmd_mode_object.ParseAppInput(app_variables, args_array, job, task, taskid2task)
	if err != nil {
		err = errors.New(fmt.Sprintf("error populate with input fields, error=%s", err.Error()))
		return err
	}

	// copy predata
	//copy(task.Predata, app_cmd_mode_object.Predata) // TODO expand variables
	//*task.Predata = *app_cmd_mode_object.Predata
	if app_cmd_mode_object.Predata != nil {
		if task.Predata == nil {
			task.Predata = make(IOmap)
		}
		for key, _ := range app_cmd_mode_object.Predata {
			task.Predata[key] = &IO{}
			*task.Predata[key] = *app_cmd_mode_object.Predata[key]
		}
	}
	// convenient dependencies (dependsOn is only used now if dependency without files is needed)
	dependsOn_map := make(map[string]bool)

	for _, dep := range task.DependsOn {
		if dep != "" {
			dependsOn_map[dep] = true
		}
	}

	for _, myio := range task.Inputs {
		if myio.Origin != "" {

			dependency := job.Id + "_" + myio.Origin
			dependsOn_map[dependency] = true
		}
	}

	//dependsOn_array := make([]string, len(dependsOn_map))
	var dependsOn_array []string
	//j := 0
	for key := range dependsOn_map {
		if key == "" {
			err = errors.New(fmt.Sprintf("error : dependsOn key is empty"))
			return err
		}
		dependsOn_array = append(dependsOn_array, key)
	}

	task.DependsOn = dependsOn_array

	for _, dep := range task.DependsOn {
		if dep == "" {
			return errors.New(fmt.Sprintf("error dep is empty !!"))
		}
	}

	logger.Debug(1, "+++ core.Expand_app_variables")
	// expand app variables in cmd_script
	// TODO rename Bash into Cmd_script

	task.Cmd.Cmd_script = make([]string, len(app_cmd_mode_object.Cmd_script))
	copy(task.Cmd.Cmd_script, app_cmd_mode_object.Cmd_script)
	logger.Debug(1, fmt.Sprintf("task.Cmd.Cmd_script (unexpanded): %s", strings.Join(task.Cmd.Cmd_script, ", ")))
	err = Expand_app_variables(app_variables, task.Cmd.Cmd_script)
	if err != nil {
		return errors.New(fmt.Sprintf("error: core.Expand_app_variables, %s", err.Error()))
	}
	logger.Debug(1, fmt.Sprintf("task.Cmd.Cmd_script (expanded): %s", strings.Join(task.Cmd.Cmd_script, ", ")))

	return
}

func (appr AppRegistry) createIOnodes(job *Job) (err error) {

	// go over tasks

	taskid2task := make(map[string]*Task)
	taskid_processed := make(map[string]bool)

	// create taskid2task
	for _, task := range job.Tasks {

		taskid_split := strings.Split(task.Id, "_")
		taskid := taskid_split[1]
		_, ok := taskid2task[taskid]
		if ok {
			err = errors.New("error: task id not unique, id=" + taskid)
			return
		}
		taskid2task[taskid] = task
		logger.Debug(1, fmt.Sprintf("--------adding to taskid2task map: %s", taskid))
	}

	for _, task := range job.Tasks {

		err = appr.createIOnodes_forTask(job, task, taskid2task, taskid_processed)
		if err != nil {
			return err
		}
	}

	logger.Debug(1, fmt.Sprintf("+++ +++ createIONodesm finished"))
	return
}

// read variables and (optionally) populate with input nodes
// 1) for reading variables, it needs only acm.Get_default_app_variables()
// 2) for populating input nodes it needs output of 2 !
func (acm AppCommandMode) ParseAppInput(app_variables AppVariables, args_array []AppResource, job *Job, task *Task, taskid2task map[string]*Task) (err error) {

	//app_variables, err = acm.Get_default_app_variables()

	if err != nil {
		return
	}

	var inputs IOmap
	//var outputs IOmap

	if job != nil {
		if task.Inputs == nil {
			task.Inputs = make(IOmap)
		}
		inputs = task.Inputs
		//outputs = task.Outputs
	}

	//reg_equal := regexp.MustCompile(`\s*=\s*`)

	for arg_position, input_arg := range args_array {
		logger.Debug(1, fmt.Sprintf("reading task input position: %d", arg_position))
		logger.Debug(1, fmt.Sprintf("resource: %s", input_arg.Resource))

		var input_variable_name = input_arg.Key // can be used by any resource
		var input_variable_value = ""

		//logger.Debug(1, fmt.Sprintf("Key: %s", input_arg.Key))
		//logger.Debug(1, fmt.Sprintf("Value: %s", input_arg.Value))

		var input_variable_type_expected = Ait_undefined

		resource_type, err := string2apptype(input_arg.Resource)

		if err != nil || resource_type == Ait_undefined {
			err = errors.New(fmt.Sprintf("app input type undefined"))
			return err
		}

		if input_variable_name == "" {
			input_variable_name = acm.Input[arg_position][1] // use position to infer key name
		}

		if input_variable_name == "" {
			return errors.New(fmt.Sprintf("error: name/key for argument not found"))
		}

		app_var, ok := app_variables[input_variable_name]
		if !ok {
			err = errors.New(fmt.Sprintf("variable \"%s\" not found", input_variable_name))
			return err
		}

		input_variable_type_expected = app_var.Var_type
		//, err = string2apptype(acm.Input[arg_position][0])
		if input_variable_type_expected == Ait_undefined {
			err = errors.New(fmt.Sprintf("app input type undefined"))
			return err
		}

		if !resource_type.HasType(input_variable_type_expected) {
			err = errors.New(fmt.Sprintf("types do not match, %s, %s", apptype2string(resource_type), apptype2string(input_variable_type_expected)))
			return err
		}

		switch resource_type {
		case Ait_shock:
			logger.Debug(1, fmt.Sprintf("processing: %s", apptype2string(resource_type)))
			filename := input_arg.Filename
			host := input_arg.Host
			node := input_arg.Node
			if filename != "" {
				input_variable_value = filename
			} else {
				//TODO invent filename ?
			}

			// TODO make sure resource_type corresponds to expected type in app def

			if job != nil {

				if _, ok := inputs[filename]; ok {
					return errors.New(fmt.Sprintf("input node already exists: %s", input_variable_name))
				}

				inputs[filename] = &IO{Host: host, Node: node, DataToken: task.Info.DataToken} // TODO set ShockFilename ?

			}

		case Ait_task:
			logger.Debug(1, fmt.Sprintf("processing: %s", apptype2string(resource_type)))

			//taskid2task

			providing_task_id := input_arg.Task
			outputPosition := input_arg.OutputPosition
			outputName := input_arg.OutputName

			// find filename
			filename := ""
			providing_task, ok := taskid2task[providing_task_id]

			if !ok {

				err = errors.New(fmt.Sprintf("did not find providing task: %s", providing_task_id))
				return err
			}

			if outputPosition != nil {
			Loop_outputPosition:
				for io_filename, my_io := range providing_task.Outputs {

					if my_io.AppPosition == *outputPosition {

						filename = io_filename
						break Loop_outputPosition
					}

				}
				if filename == "" {
					err = errors.New(fmt.Sprintf("did not find providing position \"%d\" in task \"%s\"", *outputPosition, task))
					return err
				}
			} else if outputName != "" {
			Loop_outputName:
				for io_filename, my_io := range providing_task.Outputs {
					logger.Debug(1, fmt.Sprintf("Ait_task C"))
					if my_io.AppName == outputName {

						filename = io_filename
						break Loop_outputName
					}

				}

			} else {
				err = errors.New(fmt.Sprintf("neither name nor position has been defined for providing_task_id %s", providing_task_id))
				return err
			}

			if filename == "" {
				err = errors.New(fmt.Sprintf("did not find dependency in task \"%s\"", task))
				return err
			}
			logger.Debug(1, fmt.Sprintf("Ait_task filename %s", filename))

			if job != nil {
				hostname := ""
				if job.ShockHost != "" {
					hostname = job.ShockHost // TODO check if defined
				} else {
					err = errors.New(fmt.Sprintf("job.ShockHost not defined"))
					return err
				}

				inputs[filename] = &IO{Origin: providing_task_id, Host: hostname}
			}

			input_variable_value = filename

		case Ait_string:
			logger.Debug(1, fmt.Sprintf("processing: %s", apptype2string(resource_type)))
			input_variable_value = input_arg.Value
			if input_variable_value == "" {
				return errors.New(fmt.Sprintf("no value found for variable name: %s", input_variable_name))
			}

		default:
			err = errors.New(fmt.Sprintf("Resource type unknown: %s", resource_type))
			return err
		} // end switch

		logger.Debug(1, fmt.Sprintf("from task definition: input_variable_name: \"%s\", input_variable_value: \"%s\"", input_variable_name, input_variable_value))
		// can overwrite defaults from the app-definition
		app_variables[input_variable_name] = AppVariable{Value: input_variable_value, Var_type: resource_type}

	}

	return
}

func NewVariableExpander(app_variables AppVariables) VariableExpander {

	return VariableExpander{simple_variable_match: regexp.MustCompile(`\$\{[\w-]+\}`), // inlcudes underscore
		functional_variable_match: regexp.MustCompile(`\$\{[\w-]+\:[\w-]+\}`),
		app_variables:             app_variables}
}

func (va VariableExpander) Expand(line string) (expanded string, err error) {

	replace_functional_app_variables := func(variable string) string {
		//cut name out of brackets....
		logger.Debug(1, fmt.Sprintf("f_variable: %s", variable))
		var variable_name = variable[2 : len(variable)-1]
		logger.Debug(1, fmt.Sprintf("f_variable_name: %s", variable_name))

		f_var := strings.Split(variable_name, ":")

		if len(f_var) != 2 {

			err = errors.New(fmt.Sprintf("number of colons != 2"))

			return "ERROR"
		}

		function_command := f_var[0]
		function_variable := f_var[1]

		function_variable_obj, ok := va.app_variables[function_variable]
		if !ok {
			logger.Debug(1, fmt.Sprintf("function_variable not found %s", function_variable))
			err = errors.New(fmt.Sprintf("warning: (Expand_app_variables) value of variable %s is empty: ", function_variable))
			return "ERROR"
		}
		function_variable_value := function_variable_obj.Value
		if function_variable_value == "" {
			logger.Debug(1, fmt.Sprintf("value of function_variable \"%s\" empty", function_variable))
			err = errors.New(fmt.Sprintf("function_variable_value empty"))
			return "ERROR"
		}

		logger.Debug(1, fmt.Sprintf("function_variable_value: %s ", function_variable_value))
		if function_command == "remove_extension" {
			extension := path.Ext(function_variable_value)
			//logger.Debug(1, fmt.Sprintf("extension: %s", extension))
			if extension != "" {
				function_variable_value = strings.TrimSuffix(function_variable_value, extension)
				//logger.Debug(1, fmt.Sprintf("trimmed: %s", function_variable_value))
			}
			logger.Debug(1, fmt.Sprintf("modified function_variable_value: %s", function_variable_value))
		} else {
			logger.Debug(1, fmt.Sprintf("warning: (Expand_app_variables) functional variable %s not recognized", variable))

			return variable
		}

		return function_variable_value

	}

	replace_app_variables := func(variable string) string {
		//cut name out of brackets....
		logger.Debug(1, fmt.Sprintf("variable: %s", variable))
		var variable_name = variable[2 : len(variable)-1]
		logger.Debug(1, fmt.Sprintf("variable_name: %s", variable_name))
		app_var, ok := va.app_variables[variable_name]

		if ok {

			if app_var.Value == "" {
				logger.Debug(1, fmt.Sprintf("warning: (Expand_app_variables) value of variable %s is empty: ", variable_name))
			}

			logger.Debug(1, fmt.Sprintf("app_var.Value: %s", app_var.Value))
			return app_var.Value
		}
		logger.Debug(1, fmt.Sprintf("warning: could not find variable for variable_name: %s", variable_name))
		return variable
	}

	expanded_last := line

	expanded = ""
	for expanded != expanded_last { // that should make nested variables possible ! ;-)))

		expanded = expanded_last

		expanded2 := va.functional_variable_match.ReplaceAllStringFunc(expanded, replace_functional_app_variables)
		if err != nil {
			return
		}
		logger.Debug(1, fmt.Sprintf("functional expansion: %s -> %s", expanded, expanded2))

		expanded_last = va.simple_variable_match.ReplaceAllStringFunc(expanded2, replace_app_variables)
		if err != nil {
			return
		}
		logger.Debug(1, fmt.Sprintf("simple expansion: %s -> %s", expanded2, expanded_last))
		// last for-loop should not change anything
	}

	if line != expanded {
		logger.Debug(1, fmt.Sprintf("expanded: %s -> %s", line, expanded))
	} else {
		logger.Debug(1, fmt.Sprintf("not expanded: %s", line))
	}

	return
}

func Expand_app_variables(app_variables AppVariables, cmd_script []string) (err error) {

	expander := NewVariableExpander(app_variables)

	// for all lines in cmd_script, substitute app variables
	for cmd_line_index, _ := range cmd_script {
		//cmd_script[cmd_line_index] = match.ReplaceAllStringFunc(cmd_script[cmd_line_index], replace_app_variables)
		cmd_script[cmd_line_index], err = expander.Expand(cmd_script[cmd_line_index])
		if err != nil {
			return err
		}
	}
	return
}
