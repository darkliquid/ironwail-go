package quakego

import "github.com/ironwail/ironwail-go/pkg/qgo/quake"

// Shared forward declarations for core quakego support files.
var (
	// Originally declared in client.qc.go.
	ClientObituary func(targ, attacker *quake.Entity)

	// Originally declared in plats.go.
	plat_center_touch  func()
	plat_outside_touch func()
	plat_trigger_use   func()
	plat_go_up         func()
	plat_go_down       func()
	plat_crush         func()
	train_next         func()
	func_train_find    func()

	// Originally declared in doors.go.
	door_go_down    func()
	door_go_up      func()
	fd_secret_move1 func()
	fd_secret_move2 func()
	fd_secret_move3 func()
	fd_secret_move4 func()
	fd_secret_move5 func()
	fd_secret_move6 func()
	fd_secret_done  func()

	// Originally declared in fight.go.
	knight_atk1       func()
	knight_runatk1    func()
	ogre_smash1       func()
	DemonCheckAttack  func() float32
	Demon_Melee       func(side float32)
	WizardCheckAttack func() float32
	DogCheckAttack    func() float32

	// Originally declared in weapons.go.
	SpawnBlood func(org, vel quake.Vec3, damage float32)
)
