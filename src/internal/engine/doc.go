// Package engine holds the pure tournament domain logic: formats, stages,
// standings, tiebreaks, advancement and scheduling.
//
// It performs no I/O, never reads the clock (time is passed in) and does not
// import SQLite or Wails.
package engine
