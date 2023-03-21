// Package fsm defines a finite state machine and has the ability to save this state machine to a graph file
// This graph file can be visualized using mermaid.js
package fsm

import (
	"fmt"
	"os"
	"path"
	"sort"

	"github.com/go-errors/errors"
)

type (
	// StateID represents the Identifier of the state.
	StateID int8
	// StateIDSlice represents the list of state identifiers.
	StateIDSlice []StateID
)

func (v StateIDSlice) Len() int {
	return len(v)
}

func (v StateIDSlice) Less(i, j int) bool {
	return v[i] < v[j]
}

func (v StateIDSlice) Swap(i, j int) {
	v[i], v[j] = v[j], v[i]
}

// Transition indicates an arrow in the state graph.
type Transition struct {
	// To represents the to-be-new state
	To StateID
	// Description is what type of message the arrow gets in the graph
	Description string
}

type (
	States map[StateID]State
)

// State represents a single node in the graph.
type State struct {
	// Transitions indicates which out arrows this node has
	Transitions []Transition
}

// FSM represents the total graph.
type FSM struct {
	// States is the map from state ID to states
	States States

	// Current is the current state represented by the identifier
	Current StateID

	// Name represents the descriptive name of this state machine
	Name string

	// StateCallback is the function ran when a transition occurs
	// It takes the old state, the new state and the data and returns if this is handled by the client
	StateCallback func(StateID, StateID, interface{}) bool

	// Directory represents the path where the state graph is stored
	Directory string

	// Generate represents whether we want to generate the graph
	Generate bool

	// GetStateName gets the name of a state as a string
	GetStateName func(StateID) string
}

// Init initializes the state machine and sets it to the given current state.
func (fsm *FSM) Init(
	current StateID,
	states States,
	callback func(StateID, StateID, interface{}) bool,
	directory string,
	nameGen func(StateID) string,
	generate bool,
) {
	fsm.States = states
	fsm.Current = current
	fsm.StateCallback = callback
	fsm.Directory = directory
	fsm.GetStateName = nameGen
	fsm.Generate = generate
}

// InState returns whether or not the state machine is in the given 'check' state.
func (fsm *FSM) InState(check StateID) bool {
	return check == fsm.Current
}

func (fsm *FSM) CheckTransition(desired StateID) error {
	for _, ts := range fsm.States[fsm.Current].Transitions {
		if ts.To == desired {
			return nil
		}
	}
	return errors.Errorf("fsm invalid transition attempt from '%s' to '%s'", fsm.GetStateName(fsm.Current), fsm.GetStateName(desired))
}

// graphFilename gets the full path to the graph filename including the .graph extension.
func (fsm *FSM) graphFilename(extension string) string {
	pth := path.Join(fsm.Directory, "graph")
	return fmt.Sprintf("%s%s", pth, extension)
}

// writeGraph writes the state machine to a .graph file.
func (fsm *FSM) writeGraph() {
	gph := fsm.GenerateGraph()
	gf := fsm.graphFilename(".graph")
	f, err := os.Create(gf)
	if err != nil {
		return
	}
	defer func() {
		_ = f.Close()
	}()

	// Writing the graph is best effort
	// TODO: Return string instead, we do not want to write files in this library
	_, _ = f.WriteString(gph)
}

// GoTransitionRequired transitions the state machine to a new state with associated state data 'data'
// If this transition is not handled by the client, it returns an error.
func (fsm *FSM) GoTransitionRequired(newState StateID, data interface{}) error {
	oldState := fsm.Current
	if !fsm.GoTransitionWithData(newState, data) {
		return errors.Errorf("fsm failed transition from '%v' to '%v'", fsm.GetStateName(oldState), fsm.GetStateName(newState))
	}
	return nil
}

// GoTransitionWithData is a helper that transitions the state machine toward the 'newState' with associated state data 'data'
// It returns whether or not the transition is handled by the client.
func (fsm *FSM) GoTransitionWithData(newState StateID, data interface{}) bool {
	if fsm.CheckTransition(newState) != nil {
		return false
	}

	prev := fsm.Current
	fsm.Current = newState
	if fsm.Generate {
		fsm.writeGraph()
	}

	return fsm.StateCallback(prev, newState, data)
}

// GoTransition is an alias to call GoTransitionWithData but have an empty string as data.
func (fsm *FSM) GoTransition(newState StateID) bool {
	// No data means the callback is never required
	return fsm.GoTransitionWithData(newState, "")
}

// generateMermaidGraph generates a graph suitable to be converted by the mermaid.js tool
// it returns the graph as a string.
func (fsm *FSM) generateMermaidGraph() string {
	gph := "graph TD\n"
	sf := make(StateIDSlice, 0, len(fsm.States))
	for stateID := range fsm.States {
		sf = append(sf, stateID)
	}
	sort.Sort(sf)
	for _, state := range sf {
		transitions := fsm.States[state].Transitions
		for _, transition := range transitions {
			if state == fsm.Current {
				gph += "\nstyle " + fsm.GetStateName(state) + " fill:cyan\n"
			} else {
				gph += "\nstyle " + fsm.GetStateName(state) + " fill:white\n"
			}
			gph += fsm.GetStateName(
				state,
			) + "(" + fsm.GetStateName(
				state,
			) + ") " + "-->|" + transition.Description + "| " + fsm.GetStateName(
				transition.To,
			) + "\n"
		}
	}
	return gph
}

// GenerateGraph generates a mermaid graph if the state machine is initialized
// If the graph cannot be generated, it returns the empty string.
func (fsm *FSM) GenerateGraph() string {
	if fsm.GetStateName != nil {
		return fsm.generateMermaidGraph()
	}

	return ""
}
