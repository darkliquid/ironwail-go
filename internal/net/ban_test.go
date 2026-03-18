package net

import "testing"

func TestIPBan_SingleAddress(t *testing.T) {
	var b IPBan
	if err := b.SetBan("192.168.1.100", ""); err != nil {
		t.Fatal(err)
	}
	if !b.Active() {
		t.Fatal("ban should be active")
	}
	if !b.IsBanned("192.168.1.100:26000") {
		t.Fatal("exact match should be banned")
	}
	if b.IsBanned("192.168.1.101:26000") {
		t.Fatal("different IP should not be banned")
	}
}

func TestIPBan_SubnetMask(t *testing.T) {
	var b IPBan
	if err := b.SetBan("192.168.1.0", "255.255.255.0"); err != nil {
		t.Fatal(err)
	}
	if !b.IsBanned("192.168.1.42:26000") {
		t.Fatal("IP in subnet should be banned")
	}
	if b.IsBanned("192.168.2.42:26000") {
		t.Fatal("IP outside subnet should not be banned")
	}
}

func TestIPBan_Off(t *testing.T) {
	var b IPBan
	if err := b.SetBan("192.168.1.100", ""); err != nil {
		t.Fatal(err)
	}
	if err := b.SetBan("off", ""); err != nil {
		t.Fatal(err)
	}
	if b.Active() {
		t.Fatal("ban should be inactive after 'off'")
	}
	if b.IsBanned("192.168.1.100:26000") {
		t.Fatal("should not ban when inactive")
	}
}

func TestIPBan_String(t *testing.T) {
	var b IPBan
	if s := b.String(); s != "Banning not active" {
		t.Fatalf("inactive string = %q", s)
	}
	if err := b.SetBan("10.0.0.1", "255.255.0.0"); err != nil {
		t.Fatal(err)
	}
	s := b.String()
	if s != "Banning 10.0.0.1 [255.255.0.0]" {
		t.Fatalf("active string = %q", s)
	}
}
