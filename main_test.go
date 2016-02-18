package main

import (
	"github.com/postmates/ftl/ftl"
	"testing"
)

func Test_syncPackage_downNone(t *testing.T) {
	rr := []*ftl.RevisionInfo{
		{"test", "001"},
		{"test", "002"},
		{"test", "003"},
	}

	lr := []*ftl.RevisionInfo{
		{"test", "001"},
		{"test", "002"},
		{"test", "003"},
	}

	download, purge, err := syncPackage(rr, lr, &ftl.RevisionInfo{"test", "001"})
	if err != nil {
		t.Error("Error from syncPackage", err)
	}

	if len(download) > 0 {
		t.Error("Expected no download")
	}
	if len(purge) > 0 {
		t.Error("Expected no purge")
	}
}

func Test_syncPackage_downSome(t *testing.T) {
	rr := []*ftl.RevisionInfo{
		{"test", "001"},
		{"test", "002"},
		{"test", "003"},
	}

	lr := []*ftl.RevisionInfo{}

	download, purge, err := syncPackage(rr, lr, &ftl.RevisionInfo{"test", "002"})
	if err != nil {
		t.Error("Error from syncPackage", err)
	}

	if len(download) != 2 {
		t.Error("Expected two downloads")
	}

	if len(purge) > 0 {
		t.Error("Expected no purges")
	}

	if download[0].Revision != "002" {
		t.Error("Expected 002", download)
	}
}

func Test_syncPackage_downMore(t *testing.T) {
	rr := []*ftl.RevisionInfo{
		{"test", "001"},
		{"test", "002"},
		{"test", "003"},
	}

	lr := []*ftl.RevisionInfo{
		{"test", "002"},
	}

	download, purge, err := syncPackage(rr, lr, &ftl.RevisionInfo{"test", "001"})
	if err != nil {
		t.Error("Error from syncPackage", err)
	}

	if len(download) != 2 {
		t.Error("Expected two downloads")
	}

	if len(purge) > 0 {
		t.Error("Expected no purges")
	}

	if download[0].Revision != "001" {
		t.Error("Expected 001", download)
	}
}

func Test_syncPackage_downMoreLimit(t *testing.T) {
	rr := []*ftl.RevisionInfo{
		{"test", "001"},
		{"test", "002"},
		{"test", "003"},
	}

	lr := []*ftl.RevisionInfo{
		{"test", "002"},
	}

	download, purge, err := syncPackage(rr, lr, &ftl.RevisionInfo{"test", "002"})
	if err != nil {
		t.Error("Error from syncPackage", err)
	}

	if len(download) != 1 {
		t.Error("Expected one downloads")
	}

	if len(purge) > 0 {
		t.Error("Expected no purges")
	}

	if download[0].Revision != "003" {
		t.Error("Expected 003", download)
	}
}

func Test_syncPackage_purgeNone(t *testing.T) {
	rr := []*ftl.RevisionInfo{
		{"test", "001"},
		{"test", "002"},
	}

	lr := []*ftl.RevisionInfo{
		{"test", "001"},
		{"test", "002"},
	}

	download, purge, err := syncPackage(rr, lr, &ftl.RevisionInfo{"test", "002"})
	if err != nil {
		t.Error("Error from syncPackage", err)
	}

	if len(download) > 0 {
		t.Error("Expected no downloads")
	}

	if len(purge) > 0 {
		t.Error("Expected no purges")
	}
}

func Test_syncPackage_purgeOne(t *testing.T) {
	rr := []*ftl.RevisionInfo{
		{"test", "002"},
	}

	lr := []*ftl.RevisionInfo{
		{"test", "001"},
		{"test", "002"},
	}

	download, purge, err := syncPackage(rr, lr, &ftl.RevisionInfo{"test", "001"})
	if err != nil {
		t.Error("Error from syncPackage", err)
	}

	if len(download) > 0 {
		t.Error("Expected no downloads")
	}

	if len(purge) != 1 {
		t.Error("Expected a purges")
	}

	if purge[0].Revision != "001" {
		t.Error("Expected purge 001")
	}
}

func Test_syncPackage_purgeAll(t *testing.T) {
	rr := []*ftl.RevisionInfo{
	}

	lr := []*ftl.RevisionInfo{
		{"test", "001"},
		{"test", "002"},
	}

	download, purge, err := syncPackage(rr, lr, nil)
	if err != nil {
		t.Error("Error from syncPackage", err)
	}

	if len(download) > 0 {
		t.Error("Expected no downloads")
	}

	if len(purge) != 2 {
		t.Error("Expected a purges")
	}

	if purge[0].Revision != "001" {
		t.Error("Expected purge 001")
	}
}
