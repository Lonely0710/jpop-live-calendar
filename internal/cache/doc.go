// Package cache stores the fetched public announcements in a plain JSON
// file (still no database), so a restart starts with the last sync's data
// instead of an empty calendar. Only the announcements are saved: shows,
// rounds, entries and the change log are derived from them again on load,
// which keeps the file independent of the parser's current rules.
//
// The file carries a version. A file written by another version is
// ignored, and a corrupt one is moved aside instead of stopping the app;
// the next successful sync writes a fresh snapshot.
package cache
