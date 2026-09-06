# Glossary

This glossary explains the technical terms used in the Ironwail-Go documentation, source code, and commit messages. It is written for readers without a background in 3D graphics, mathematics, Quake modding, or computer science.

Each entry starts with a term. Then follows a plain sentence or two. Where a term has a common short name, that name appears in the heading. Code names, such as `qbsp` and `PVS`, keep their original spelling.

The sections follow the natural order of the project: math, graphics, the Quake engine, QuakeC, the map compilers, file formats, and general computer science.

## Contents

- [Math and 3D coordinates](#math-and-3d-coordinates)
- [3D graphics and the renderer](#3d-graphics-and-the-renderer)
- [The Quake engine](#the-quake-engine)
- [QuakeC and the game VM](#quakec-and-the-game-vm)
- [The map compiler pipeline](#the-map-compiler-pipeline)
- [File formats and the virtual file system](#file-formats-and-the-virtual-file-system)
- [Computer science terms](#computer-science-terms)
- [The web build](#the-web-build)

## Math and 3D coordinates

- AABB: Axis-aligned bounding box. A box with no rotation, described by its two opposite corners. Collision checks against an AABB are fast.
- Angles: The facing of an object as three rotations, one per axis: pitch, yaw, and roll.
- Aspect ratio: The width of a viewport divided by its height.
- Axis-aligned: Aligned with the world axes, not rotated.
- Clamp: To limit a value to a range, cutting it off at the ends.
- Convex: A shape where any straight line between two points inside it stays inside. BSP leaves are convex, which keeps plane tests simple.
- Coordinate system: The agreed set of axes that names positions. A position only means something in one system.
- Camera space: Coordinates measured from the camera position and facing. Also called eye space or view space.
- Clip space: Coordinates after the projection matrix, before the GPU divides by depth.
- Model space: The coordinates of a model as it was built, before it is placed, rotated, or scaled.
- Screen space: The final pixel coordinates on the render target.
- World space: The fixed coordinate system of the map, in Quake units.
- Cross product: An operation on two vectors that returns a third vector at a right angle to both. It computes surface normals.
- Dot product: An operation on two vectors that returns a single number. It measures how aligned the vectors are. Lighting and plane tests use it.
- Euler angles: Rotation stored as three separate angles, one per axis. See Angles.
- Exponential: Growing or shrinking by a constant factor per step. Distance fog and sound attenuation use it.
- Far plane: The distance from the camera where drawing stops. Geometry beyond it is cut.
- Field of view (FOV): The visible angle of the scene, in degrees. Quake uses 90 by default.
- Float32: A number stored with about 7 digits of precision. Quake math uses float32 to match the C engine and the GPU.
- Frustum: The truncated pyramid of space the camera sees. It is small near the camera and wide at the far plane.
- Halfspace: One side of a plane. A convex shape is the overlap of many halfspaces.
- Infinity: A special float value larger than any finite number. It can come from dividing by zero.
- Inverse matrix: The reverse of a matrix. Multiplying by it undoes the original transform.
- Lerp: Short for linear interpolation. To compute a value partway between two others using a fraction from zero to one.
- Magnitude: The length of a vector.
- Matrix: A table of numbers that transforms coordinates. A 4 by 4 matrix transforms 3D points.
- Mins and maxs: The two opposite corners of a bounding box. Mins holds the lowest values, maxs holds the highest.
- Model matrix: The matrix that places a model: it moves its local coordinates into world coordinates with position, rotation, and scale.
- NaN: Not a number. A special float value from invalid math, such as zero divided by zero.
- Near plane: The distance from the camera where drawing starts. Geometry closer is cut.
- NDC: Normalized device coordinates. Clip space after the perspective divide, from -1 to 1 on each axis.
- Normalize: To scale a vector to length 1 without changing its direction.
- Perspective divide: Dividing clip coordinates by their depth. It makes distant objects smaller.
- Perspective-correct interpolation: Blending values across a triangle while accounting for depth, so textures do not slide.
- Pitch: The rotation of looking up and down.
- Plane: An infinite flat surface that divides space in two. It is defined by a normal vector and a distance.
- Plane distance: How far the plane sits from the origin along its normal.
- Plane normal: The vector that points straight out of a plane, at a right angle to its surface.
- Precision: How exactly a number is stored. Float32 loses detail far from zero.
- Projection matrix: The matrix that turns camera coordinates into clip space and adds the perspective effect.
- Quantization: Rounding a value to fewer steps so it fits in fewer bytes. Quake stores angles in 256 steps and origins in eighths of a unit.
- Quaternion: Rotation stored as four numbers. It blends between rotations more smoothly than Euler angles.
- Right-handed: A rule for 3D axes. In Quake, crossing X with Y gives Z. Forward with left gives up.
- Roll: The rotation of tilting the head sideways.
- Scalar: A single number, as opposed to a vector.
- Sine: A smooth repeating wave function. Quake uses it to warp water surfaces and animate lights.
- Transform: A combination of position, rotation, and scale that moves coordinates from one space to another.
- Unit vector: A vector of length 1. Direction-only values are stored this way.
- Vector: A list of numbers that names a position or a direction. In Quake it has three parts: X, Y, and Z.
- View matrix: The matrix that turns world coordinates into camera coordinates, making the camera the center of everything.
- VP matrix: The view matrix multiplied by the projection matrix. It turns world coordinates straight into clip space.
- Yaw: The rotation of looking left and right.
- Z-fighting: A flicker where two surfaces sit at the same depth and neither wins the depth test. See Depth test.

## 3D graphics and the renderer

### Geometry and buffers

- Edge: The straight line between two points at the border of a polygon.
- Face: A flat polygon of a surface. The renderer draws faces as triangles.
- Index: See Index in computer science terms. In graphics, an index names one vertex in the vertex buffer.
- Index buffer: A list of numbers on the GPU. Each number points into the vertex buffer, so triangles can reuse vertices.
- Index triple: Three indexes that name the three vertices of one triangle.
- Mesh: A set of triangles that forms a model or a surface.
- Offset: See Offset in computer science terms. In a vertex record, the offset names one field.
- Phantom triangle: A wrong triangle that appears where no real geometry exists, usually from corrupt vertex data.
- Polygon: A flat shape with three or more edges. Quake faces are polygons.
- Stride: The byte distance from the start of one vertex record to the start of the next.
- Triangle: The basic drawable unit of 3D graphics. Three vertices make one triangle.
- Vertex: One corner point of a 3D shape. It carries a position and extra data such as texture and light values.
- Vertex buffer: The block of GPU memory that holds the vertex records.
- Vertex contract: The agreed byte layout of one vertex, shared by the packing code, the buffer layout, and the shader.
- Vertex packing: Converting vertex records into the flat bytes the GPU expects.

### Textures, images, and materials

- Alpha: The opacity part of a color. Zero is fully see-through, one is fully solid.
- Alpha test: Dropping a pixel when its alpha is below a fixed threshold. It makes hard cutouts such as grates and fences.
- Alternate animation: A second animation sequence for a texture or model that replaces the first one.
- Ambient light: A uniform base light level so surfaces are never completely black.
- Animated texture: A Quake texture with several frames that cycle over time, such as water or lava.
- Atlas page: One layer of a texture array that holds part of an atlas.
- Bin packing: Placing many rectangles into one larger rectangle with little wasted space.
- Filtering: How the GPU blends texels when a texture is scaled up or down.
- Fullbright: Texture colors that ignore light and glow at full brightness, such as lava and torches.
- Image formats: The file kinds the engine reads, such as PNG, TGA, and PCX. Quake images are small and simple.
- Linear filtering: Blending neighboring texels smoothly. It prevents blocky scaling but can bleed across atlas borders.
- Mip level: One copy in a chain of pre-shrunk texture images, each half the size of the last.
- Mipmap: The whole chain of mip levels. The GPU picks the right size for the distance.
- Nearest filtering: Picking the single closest texel. It stays crisp and blocky, good for UI art.
- Palette: A fixed table of 256 colors. Quake images store one byte per pixel, and that byte names a color in the palette.
- Palette index: One byte that names a color in the palette.
- Palette.lmp: The 768-byte file that defines the 256 colors of the Quake palette.
- Pixel: One dot of a screen image.
- QPic: A WAD lump for a 2D image such as menu art, stored as width, height, and palette-indexed pixels.
- Sampler: The GPU object that controls texture reading: which filtering and how edges behave.
- Scrap atlas: A small atlas for UI textures such as console and menu graphics, packed tightly.
- Sky brush: A world brush textured with a sky texture. The engine fills its region with the sky instead of drawing it.
- Sky texture: A special world texture that stands for the sky. Faces with it are not drawn as solid walls.
- Skyline bin packing: A packing method that tracks the filled skyline and places each new rectangle at the lowest fitting spot.
- Texel: One color sample of a texture.
- Texture: A 2D image applied to surfaces. Shaders read it to color pixels.
- Texture array: A stack of same-sized texture layers on the GPU. The shader picks a layer by number.
- Texture atlas: One large texture holding many smaller images side by side. A UV offset selects which image a surface uses.
- Texture view: A GPU object that points at one image inside a larger texture resource.
- Transparent index: Palette index 255, reserved as see-through in Quake images.
- UV coordinates: Two numbers that pick a point on a texture. U runs across, V runs down. Also called texcoords.
- WAD: The archive format that holds textures and pictures. See File formats.

### Lighting and lightmaps

- Colorbleed: The tint a colored surface gives to nearby surfaces during radiosity baking.
- Dynamic light: A moving light in the world, such as a muzzle flash. The engine renders them as additive overlays or clusters them in a grid.
- Fog: A color mixed into distant surfaces so they fade into the distance.
- Exponential fog: Fog that grows stronger with distance along an exponential curve. Quake uses this style.
- Fog density: How quickly fog covers distance. The cvar value is converted into the shader formula.
- Gamma: A brightness curve applied to the final image. Humans see dark values non-linearly, so the output is adjusted.
- Lightgrid: A coarse 3D grid of light samples that tints entities and other non-world objects. It is a known gap in this project.
- Lightmap: A small grayscale image per face that stores the baked light. The shader multiplies the texture by the lightmap.
- Lightmap page: One layer of the lightmap texture array. Each page holds lightmaps for a group of faces.
- Lightstyle: A numbered light animation. Each style holds a string of intensity values, one per frame. Style 255 means no lightmap at all.
- Lightstyle scalar: The current intensity of one light style for this frame, computed from its string. Shaders multiply lightmap samples by it.
- Light source: A point in the world that casts light, such as a torch or a light entity.
- Luxel: One sample of a lightmap. The map compiler places one every 16 units along the face.
- Overbright multiplier: A scale factor, usually 2, applied to lightmap samples when rendering to recover brightness that byte storage loses.
- Radiosity: Simulating light that bounces between surfaces. Each lit surface re-emits light onto its neighbors.
- Shadow trace: A ray cast from a light toward a surface. If geometry blocks it, that point sits in shadow.
- Sun: A light with no position that shines from one direction, like the sun. A flag adds sun lighting to a map.
- Supersampling: Computing more than one sample per luxel and averaging them. It smooths lighting edges.

### The graphics pipeline

- Additive blending: Adding the new color to the target. Glow and light effects use it.
- Alpha blending: Mixing a new color with what is already drawn, using alpha as the weight.
- Attachment: An image a render pass reads or writes, such as a color, depth, or stencil image.
- Barrier: A synchronization point where the GPU waits, so later work sees earlier work. One example is a texture changing from drawn-on to read-from.
- Bind group: A bundle of GPU resources, buffers, textures, and samplers, tied to a pipeline for a draw.
- Bind group layout: The declared shape of a bind group: what each slot holds.
- Blend state: The rules for mixing new colors with the target: the factors and the operation.
- Blit: Copying a block of pixels from one image to another.
- Command buffer: A recorded list of GPU commands. The CPU builds it, and the GPU plays it back later.
- Command encoder: The recorder that builds the command buffer.
- Composite pass: The final post-processing pass that combines the scene and effects into one image.
- Compute pass: A section of the command buffer that runs compute shaders.
- Compute shader: A shader that runs general math, not drawing. It can write to buffers and textures.
- CPU: The central processor. It prepares work for the GPU.
- Descriptor: The GPU record that points at a resource such as a buffer or texture.
- Descriptor pool: The memory pool that GPU descriptors come from.
- Descriptor set: The group of descriptors bound for a draw. It is the native layer under a bind group on Vulkan.
- Descriptor set layout: The declared order and types of the descriptors in a set.
- Dispatch: The command that launches a compute shader over a grid of workgroups.
- Draw call: One command to the GPU: draw a set of triangles with a chosen pipeline.
- Dynamic offset: Binding one buffer once and pointing a draw at a different offset into it, so many draws share one binding.
- Fragment shader: The shader that runs for every pixel of a triangle and computes its color.
- Fullscreen triangle: One big triangle drawn to cover the whole render target. Post-processing passes use it.
- GPU: The graphics processor. It runs many small jobs in parallel.
- MRT: Multiple render targets. A render pass that writes several images at once.
- Offscreen render target: A render target that is not the window. The scene renders there first, then it is processed and shown.
- Overlay: The 2D layer drawn on top of the 3D scene: the HUD, the menu, and the console.
- Pipeline: The assembled GPU setup for drawing: shaders, vertex layout, and state such as depth and blending.
- Pipeline layout: The declared order of bind groups and other data for a pipeline.
- Polyblend: The Quake fullscreen color wash. Damage flashes and underwater tint use it.
- Post-processing: Passes that run after the 3D scene to change the final image.
- Present: Showing a finished image on the window by swapping the swapchain.
- Readback: Copying GPU data back to the CPU, used for screenshots and debug checks.
- Render pass: A section of commands that draws into one set of images. It starts by loading or clearing them.
- Render target: An image that a render pass draws into.
- Shader: A small program that runs on the GPU in parallel, one per vertex, per pixel, or per work item.
- Shader module: The compiled form of a shader program.
- Staging buffer: A CPU-visible buffer used to move data between the CPU and the GPU.
- Storage buffer: A large GPU buffer that shaders read and write at arbitrary offsets. Big lists such as materials live here.
- Surface view: The image of the window that the renderer draws into for this frame.
- Swapchain: The set of images the window owns and shows in turn. The GPU draws to one while another is displayed.
- Uniform buffer: A small GPU buffer of fixed data, such as matrices, readable by shaders during a draw.
- Vertex shader: The shader that runs for every vertex and outputs its final clip position.
- Water warp: The screen distortion Quake applies when the camera is underwater.
- Workgroup: A group of compute threads that run together and can share fast memory.

### Depth, stencils, and drawing order

- Back-to-front sorting: Drawing far translucent surfaces before near ones, so blending looks right.
- Depth bias: A small constant added to depth to stop z-fighting.
- Depth buffer: An image that stores the distance to the nearest drawn surface for each pixel. It is also called the z-buffer.
- Depth test: Drawing a pixel only when its depth is closer than the stored depth.
- Depth write: Whether the pass updates the stored depth.
- Late translucency: Translucent work done after the main scene, which costs extra passes and sorting.
- Load and store operations: Two settings per attachment in a render pass. Load says whether to keep existing content or clear it. Store says whether to keep the result.
- Occlusion culling: Skipping objects hidden behind other objects.
- OIT: Order-independent transparency. A group of methods that draw translucent surfaces in any order and still get the right result.
- Opaque: Fully solid, not see-through. Opaque drawing order does not matter because of the depth test.
- Overdraw: Drawing the same pixel several times, which wastes GPU work.
- Revealage target: The OIT image that stores how much background is still uncovered.
- Accumulation target: The OIT image that stores weighted color from translucent surfaces.
- Stencil buffer: An extra image of per-pixel masks. It can block drawing where the mask says no.
- Translucent: Partly see-through. Translucent surfaces must blend, so their draw order matters.
- Viewport: The rectangle of a render target that receives drawing.
- Weighted-blended transparency: An OIT method each translucent draw adds weighted color to the accumulation target, multiplies the revealage target, and a final fullscreen pass combines them.

### Seeing the world

- Billboard: A flat quad that always faces the camera. Sprites are billboards.
- Camera: The point and facing that define what the player sees.
- Camera origin: The position of the camera, usually the player origin plus the eye height.
- Cubemap: Six images arranged as the faces of a cube around the camera. Skyboxes use them.
- Frustum culling: Skipping objects that are fully outside the frustum.
- Lookup leaf: The leaf a point sits in, found by walking the BSP tree. See PointInLeaf.
- Occlusion culling: See Depth, stencils, and drawing order.
- Portal pop-in: A visible jump where geometry appears at a portal boundary because visibility data is missing.
- Skybox: A background built from six images around the camera. External skyboxes load PNG, TGA, or JPG files.
- Sprite: A 2D image drawn as a billboard.
- View bob: A small rhythmic camera motion while walking.
- Viewheight: How high the camera sits above the player origin. Quake uses 22 units, about 56 centimeters.
- Viewmodel: The first-person model of the held weapon, drawn in front of the camera.
- Viewmodel fudge: A small manual adjustment to the viewmodel position for game feel.
- View punch: A temporary camera tilt added when the player takes damage. Also called damage kick.

## The Quake engine

### The client and the server

- Attract mode: The demo loop shown at the main menu. The game queues demo names and plays them in turn.
- Authoritative: Holding the true state. The server is authoritative. The client renders what the server reports.
- Client: The engine part that runs on the player side. It parses server messages, predicts its own movement, and draws.
- Client parser: The code that reads server messages and updates the client state.
- Client prediction: The client running a copy of its own movement so input feels instant, then correcting from server messages.
- Console: The text command line opened with the tilde key. It accepts typed commands and shows engine messages.
- Cvar: Console variable. A named engine setting, readable and writable from the console.
- Datagram: The per-frame unreliable packet that carries the moving world state.
- Delta time: The seconds since the last frame. Movement scales by it. Also called frametime.
- Demo: A recording of the server messages from a match. The engine plays it back for demo playback.
- Frame: One pass of the game loop. The engine advances time and runs client and server once per frame.
- HUD: Heads-up display. Text and numbers drawn over the game view, such as health and ammo.
- Host: The engine part that owns both the client and the server and drives the frame loop.
- Intermission: The break between maps that shows the scoreboard.
- Key destination: The switch that decides where key presses go: the menu, the console, or the game.
- Loading plaque: The screen shown while a map loads.
- Menu overlay: The menu drawn over the game world.
- Server: The engine part that runs the simulation: physics, QuakeC, and message sending.
- Spectator: A client that watches the game but does not play.
- SrvTime: The current game time of the server, in seconds.
- Status bar: The bottom bar of the HUD in Quake. It shows health, armor, ammo, and weapons. Also called the sbar.
- User command: The block a player sends each frame with pressed buttons, movement, and aim angles.

### Entities and the game world

- Blocked: The entity function that runs when a pusher cannot move a blocking entity, such as a door crushing a player.
- Brush model: World geometry used by an entity. Doors and platforms are brush models.
- Classname: The text name of an entity type, such as `func_door`. At load, the engine maps it to the matching spawn function.
- Edict: The engine record of one entity. The name comes from entity dictionary. The game calls entities edicts.
- Entity: A thing in the game world: a player, a monster, an item, a door, a trigger, or the level itself.
- EntVars: The standard set of fields every entity has, such as origin, velocity, and health.
- Frames: The animation poses of a model. An MDL stores a set of frames. Also called animation frames.
- Groundentity: The entity an entity stands on, if any.
- Impulse: A one-byte command inside a user command, used for weapon selection and other quick actions.
- MakeStatic: Turning an entity into a permanent, non-interactive part of the world. See Static entity.
- Modelindex: The number of an entity model in the precache list. Messages carry this number, not a file name.
- Movetype: The rule that decides how an entity moves each frame. Quake has none, walk, fly, toss, push, noclip, and others.
- NPC: A non-player character, such as a monster or a friendly creature. The Quake docs call them monsters.
- Nextthink: The game time at which an entity think function runs. Setting it schedules the think.
- Origin: The position of an entity in world space.
- Player: The client-controlled entity.
- Precache: Loading a model or sound before the map starts and storing it at a fixed index. The index travels in messages instead of the file name.
- Precache slot: The fixed index of a precached model or sound.
- Pusher: A moving brush entity, such as a door or platform. It carries anything standing on it. Pushers run on local time so they can pause on their own.
- SetSize: The builtin that sets the bounding box of an entity for physics.
- Skill: The difficulty of a map, from 1 (easiest) to 3 (hardest). Spawn flags can filter entities by skill.
- Skin: Which color set of a model an entity uses. Players use it for shirt and pants colors.
- Solid type: How the engine treats the box of an entity during collision: not solid, a solid box, or a trigger box.
- Spawn: To create an entity.
- Spawn function: The QuakeC function that builds one entity type from its map keys when a map loads.
- Spawnflags: A bit field on an entity in the map. Spawn code reads it to change how the entity is created.
- Static entity: An entity turned into a permanent part of the world at map load. It never changes or updates.
- Temp entity: A short-lived visual effect such as an explosion, a bolt of lightning, or a spike. Also called a temporary entity.
- Think: The entity function that runs at a scheduled time.
- Touch: The entity function that runs when the entity touches another entity.
- Trail origin: The previous position of an entity. Rocket trails draw a segment from the old position to the new one.
- Trigger: An invisible volume that runs a use or touch action when something enters it.
- Use: The entity function that runs when another entity fires the entity through the target system.
- View offset: The eye position relative to the entity origin.
- Water level: How deep an entity sits in liquid. It drives swimming and drowning.
- Water type: Which liquid an entity is in, such as water, slime, or lava.
- Worldspawn: Entity zero, the map itself. Its keys set global settings such as fog and water alpha.

### The BSP map and visibility

- BSP: Binary space partitioning. A method of splitting space with planes into a tree, so the engine can find what matters quickly.
- BSP tree: The tree of planes that splits the map. Each branch splits one region into two.
- Clipnode: A node in the collision version of the BSP tree. Each hull has its own clip tree built from the same planes.
- Cluster: A group of leaves that share one visibility footprint. The cluster form of the portal file groups leaves this way.
- Contents: The material of a space in the map: empty, solid, water, slime, lava, or sky. Physics and clipping check contents.
- Edge: See the graphics section.
- Headnode: The index of the top clipnode for a hull. Each hull has its own headnode.
- Leaf: A convex volume at the end of the BSP tree. The map is a stack of convex rooms.
- Leaf zero: The one shared solid leaf. Every point inside solid geometry lives in it.
- Marksurfaces: The list of faces that touch a leaf. The engine uses it to collect the visible faces of the current leaf.
- Miptex: The texture table entry in the BSP. It holds the name, the size, and the four mipmap offsets.
- Miptex table: The list of texture entries in a BSP, referenced by the texinfo records.
- Node: A decision point in the BSP tree. It splits space with a plane and points to two children.
- Plane: See Math and 3D coordinates.
- PointInLeaf: The search that walks the BSP tree from the root to find which leaf a point is in.
- Portal: The polygon between two adjacent leaves where visibility can flow from one side to the other.
- Portal file: The file the vis tool reads, listing every portal with its outline and the two leaves it joins. See the compiler section.
- PVS: Potentially visible set. For one leaf, the list of leaves that can be seen from it, stored as a bitmask. The engine skips anything not in it.
- Sphere culling: A fast test of whether a region touches a sphere, using the stored bounds of the region.
- Submodel: One model inside the BSP. Model zero is the world. Later models are brush entities, named `*1`, `*2`, and so on in the map.
- Surfedge: A reference to an edge from one side. The BSP stores edges once and lists which faces use them.
- Texinfo: The record that ties a face to a texture. It holds the two UV axes used to project the texture onto the face.
- Visleafs: The count of visibility leaves for a model. Only world leaves carry PVS rows.
- Visofs: The offset of a leaf PVS row inside the visibility lump. A value of -1 means no visibility data.
- Winding: A loop of points that traces the outline of a polygon. Compilers and vis use windings.

### Collision and movement

- Air control: The small steering a player can do in the air. It enables strafe jumping.
- Areanode: One cell of a coarse tree that covers the map in space. The engine uses it to find which entities can touch.
- Box hull: A collision volume shaped like a box around an entity.
- CheckBottom: A check of whether an entity stands on a solid surface below it.
- Friction: The slowdown applied to sliding entities on the ground.
- Hull: A simplified solid used for collision. Quake moves a box through the map and checks it against planes. There are three hulls for point, player, and large monster sizes.
- LinkEdict: Putting an entity into the map spatial tree according to its bounds.
- Noclip: A cheat mode where the player flies through walls with no collision.
- Point contents: The material of the map at a single point. See Contents.
- Recursive hull check: The recursive walk over clipnodes that tests box collision against the map.
- Stair-step: Walk movement that tries to step up a small height, move forward, then drop. It lets players climb steps.
- Strafe jump: A movement trick that uses air control to gain speed.
- Sweep: Moving a volume along a path and checking for collisions. It is the general form of a trace.
- TestEntityPosition: A check of whether an entity box fits at a spot without touching solid.
- Touchlinks: The list of entities near an entity, used to run touch checks.
- Trace: A query that moves a point or a box from a start to an end and reports the first solid contact. The word also names the query result.
- Trace fraction: How far along the ray the trace traveled before hitting, from zero to one.
- Unstick: Nudging an entity out of solid when it ends a move stuck inside it.
- Walk move: The full ground movement code for the player and walkers.

### Server messages and the client

- Baseline: The recorded starting state of an entity. The server sends later frames as changes from this state.
- Broadcast: Sending a message or packet to every client.
- Button bits: The set of held keys in a user command, packed as bits, such as forward, back, jump, and attack.
- Clientdata: The per-frame block of a player state that drives the HUD, sent to that player.
- Delta encoding: Sending only the fields that changed since the last update, with a mask naming which fields changed.
- Entity delta: The set of changed fields for one entity in a frame update.
- MSG destinations: The routing modes for engine messages. ONE sends to one client only. ALL and BROADCAST send to all. INIT writes to the signon buffer.
- Signon: The ordered handshake between server and client before play starts. It carries the map name, precache lists, and world state.
- Signon buffer: The stored server messages from the signon handshake, such as precaches and baselines, sent to a new client.
- Spawn parms: Per-client values such as flags and team, passed to QuakeC when a player joins.
- Stat: A small number the server sends to update one HUD value, such as health or ammo.
- Static sound: A sound placed in the world at map load that plays on its own, such as ambient noise.

### Networking and the wire protocol

- Acknowledgement: A packet that says a reliable message arrived.
- Broadcast query: A packet sent to every machine on the local network, asking for Quake servers.
- Byte order: See Endianness in computer science terms.
- CLC: Client to server. The prefix for message types a client sends to the server.
- Colormap: The shirt and pants colors of a player, used to tint their model.
- Control message: A packet used for connection setup, such as a connect request or a server info reply.
- Coop: Cooperative multiplayer. Players share the single-player maps together.
- Deathmatch: Free-for-all multiplayer combat.
- Fragmentation: Splitting a large reliable message into several packets, then rebuilding it at the far end.
- Frags: The kill score in deathmatch.
- Handshake: The control messages exchanged when a client connects. The client asks to join, and the server accepts or rejects.
- Lag: The delay between a player action and its effect, driven by ping.
- Lerp flags: Network flags that control how the client interpolates an entity, such as resetting on a teleport.
- Loopback: A network driver that sends packets to the same process through memory. Single player uses it.
- MTU: Maximum transmission unit. The largest packet a network link can carry without splitting. Quake stays under it.
- NetQuake: The base Quake network protocol, version 15.
- Packet: One unit of data sent over the network.
- Ping: The round-trip time to the server in milliseconds, shown in the scoreboard.
- Protocol: The agreed byte format of messages between client and server. Quake has several versions.
- Protocol flag: A negotiated option between client and server, such as 8-bit angles, float angles, or 24-bit coordinates.
- Quake port: The default UDP port of Quake, 26000.
- Reliable message: A message the network layer resends until the receiver confirms it.
- Retransmission: Sending a reliable message again when the receiver has not confirmed it.
- RMQ: Re-Make Quake. A protocol version 999 that adds float coordinates and entity scale.
- Server browser: The in-game list of LAN servers, found with broadcast queries.
- Sequence number: A counter on packets that lets the receiver order them and notice losses.
- Stop-and-wait: Sending one reliable message at a time and waiting for its acknowledgement before the next.
- SVC: Server to client. The prefix for message types the server sends to clients.
- Teamplay: A rule set that changes damage between players so teams can fight.
- UDP: A network protocol with no guaranteed delivery. Quake uses it for speed and adds its own reliability.
- Unreliable message: A message sent once per frame with no guarantee. Missing one is fine because the next frame replaces it.
- Wire format: The byte layout of data sent over the network.

## QuakeC and the game VM

- Builtin: A Go function the engine exposes to QuakeC, such as moving an entity or playing a sound.
- Bytecode: Compiled instructions that a VM runs. They are compact and portable, unlike native machine code.
- Call stack: The stack of active function calls in the VM, one frame per call.
- CSQC: Client-side QuakeC. Optional logic that runs on the client and can draw its own HUD.
- Entry points: The named functions a program must provide, such as Init, Shutdown, and DrawHud.
- Extension fields: Extra entity fields beyond the standard set, negotiated between the VM and the engine.
- Field offset: The byte position of one entity field inside the entity data area. The VM reads fields by offset.
- Function table: The list of functions in a compiled program, mixing bytecode functions and builtins.
- Globals: The shared variables of the VM. They live in one flat array of floats. `time`, `self`, and `world` are globals.
- Instruction pointer: The position of the next bytecode statement.
- MakeVectors: A builtin that turns an angles vector into three direction vectors: forward, right, and up.
- Opcode: One instruction in the bytecode, such as add, call, or branch.
- Other: The global that names the second entity in an interaction, such as the one that touched or attacked.
- Prog version: The version number a compiled program declares. The engine rejects a mismatch.
- Progs.dat: The compiled file of QuakeC bytecode that a mod ships. The engine loads it when a game starts.
- QCVM: QuakeC virtual machine. The engine component that runs the bytecode.
- QuakeC: The game logic language of Quake. It is compiled to bytecode and run by the engine VM.
- QuakeGo: A dialect of Go that compiles to QuakeC bytecode, written with the project own compiler.
- Self: The global that names the entity running the current function.
- Source map: A map from bytecode statements back to source lines, used by debug tools.
- SSQC: Server-side QuakeC. The game logic that runs on the server.
- Stack: See Call stack.
- Statement: The QCVM instruction unit: an opcode plus three operands.
- String table: The list of text values in a compiled program. Bytecode stores a string as an index into this table.
- Trace globals: The result of the last trace: how far it went, where it ended, the plane it hit, and what it hit.
- VForward, VRight, VUp: The three direction globals that MakeVectors fills.
- World: The global that names the world entity, entity zero.

## The map compiler pipeline

- Brush: A solid shape made of six or more flat sides. Levels are carved from brushes.
- BSP2: A BSP variant with 32-bit indexes, for very large maps.
- CSG: Constructive solid geometry. Merging solid shapes into one world while resolving overlaps.
- Detail brush: A brush tagged as detail, such as a crate. It does not split the BSP, which keeps the tree simpler.
- Deterministic: Producing identical results for identical input. The compilers aim for byte-identical output on repeat runs.
- Extents: The lightmap grid size of a face, fixed so one luxel covers 16 units.
- Facet: One flat piece of a leaf region, defined by the polygon and the plane it lies on.
- Falloff: How light loses strength with distance, as light divided by distance squared.
- Flood fill: Visiting every connected space from a start point. The leak check floods from the void and marks reachable space.
- Func_detail: The entity that marks one or more brushes as detail.
- Halfspace: See Math and 3D coordinates.
- Hermetic: A test that needs no outside files or network, because its inputs are embedded in the test.
- Hull generation: Building clipnode trees for hull one and hull two from the brush planes, so the engine can collide boxes.
- Leak: A path from inside the map to the void outside the world. A leaked map cannot pass the vis stage. The compiler writes a `.pts` trail that shows the path.
- Light tool: The tool that bakes lightmaps. It casts rays from every light and stores the results per luxel. Also called qrad.
- Lightmap extents: See Extents.
- Lightmap sample: One stored light value for a luxel, a byte for a monochrome lightmap.
- Lmscale: A per-surface scale on luxel density. Smaller values give finer lightmaps.
- MakeFaces: The pass that builds the face, edge, and vertex data of the BSP from the pruned tree.
- Map file: A text file that describes a level with brushes and entities. Quake editors save it. The QuakeEd and Valve 220 syntaxes differ in how they store brushes.
- Max map limits: The fixed ceilings of the BSP formats, such as the maximum number of nodes. Compilers stop with an error at them.
- Outside distance: For each leaf, the shortest distance to the void. Leak trails walk along decreasing values to the exit.
- Parity harness: A test setup that runs the Go tool and the reference tool on the same map and compares the output.
- Phong shading: Smoothing lighting across a face by blending normals at shared vertices, so flat faces look curved.
- Prune nodes: Removing BSP nodes that contain no solid, leaving a leaner tree.
- Qbsp: The tool that turns a `.map` file into a playable BSP. It builds geometry, collision hulls, and checks for leaks.
- QLIT: The format of the `.lit` file, headed by the bytes `QLIT` and a version.
- Radiosity: See Lighting and lightmaps.
- RLE: Run-length encoding. A way to shrink repeated data: a run of zeros becomes a count. PVS rows use it.
- Shadow trace: See Lighting and lightmaps.
- Solidbsp: The classic brush-splitting BSP method. Each brush splits earlier geometry until all space is convex.
- Split policy: The rule for choosing which plane splits a region next. AUTO, FAST, and PRECISE balance speed and quality.
- Structural brush: A brush that does split the BSP and forms the map solid layout.
- Styles: The numbered lightmap slots a face can carry. Each active style gets its own lightmap.
- Supersampling: See Lighting and lightmaps.
- T-junction: A crack-prone spot where one face edge meets the middle of another face edge. Fixing splits the longer face there.
- Texinfo: See The Quake engine.
- Vis tool: The tool that computes visibility. It reads the portal file and writes the PVS into the BSP.
- Windowing: The portal-flow method of clipping visibility polygons as they pass through portals.
- Winding: See The Quake engine.

## File formats and the virtual file system

- Archive: A single file that holds many files, such as a PAK or a WAD.
- Base game directory: The main game data directory, called `id1` in Quake.
- Byte range: A span of a file marked by a start offset and a length.
- Central directory: The index at the end of a PAK or WAD: a list of names, offsets, and lengths.
- Checksum: A number computed from data. Any change to the data changes the number.
- CRC: Cyclic redundancy check. A kind of checksum. The engine uses it to validate files such as `progs.dat`.
- Gamedir: A per-mod game directory, such as `hipnotic`. The engine searches it before the base game.
- Header: The fixed first part of a file that describes the rest, such as the lump table.
- Little-endian: A byte order where the small end of a number comes first. Quake files are little-endian.
- Loose files: Files on disk, not inside an archive. The VFS treats them as one mount source.
- Lump: One named block of data inside a BSP file, listed with an offset and a length in the header.
- Magic: Fixed bytes at the start of a file that identify its format, such as `PACK` or `QLIT`.
- Mount: Adding a game directory or a PAK into the VFS search stack.
- Mount stack: The ordered list of mounted sources. Later mounts win when several hold the same file.
- Null-terminated string: Text in a file that ends at the first zero byte.
- Override order: The order in which game dirs and PAKs are searched. Mods override the base game.
- PAK: The Quake archive format. Files sit one after another, and a central directory at the end lists them. `pak0.pak` is the main base archive.
- Path sanitization: Cleaning a file name so it cannot escape its folder, such as blocking `..` entries.
- Search path: The ordered list of places the VFS looks for a file.
- Sidecar file: A file that sits next to another and adds data to it, such as the `.lit` lightmap file next to a BSP.
- Slash-normalized: File names stored with forward slashes and matched case-insensitively.
- VFS: Virtual file system. The layer that reads game assets by Quake path instead of OS paths, covering loose files and PAKs.

## Computer science terms

- Alignment: Placing data at memory addresses with a set spacing. Misaligned reads are slow or invalid.
- Array: A fixed-size list of same-typed values.
- Arena: A memory pool that hands out blocks by moving a pointer forward. Freeing happens all at once when the pool resets. Also called a bump allocator.
- Backend: The hidden implementation behind a common interface, such as the Vulkan backend of a graphics layer.
- Big-O: The way work grows with input size, stated as a class such as linear or quadratic. It is used to compare algorithms.
- Bit: One binary digit, zero or one.
- Bitmask: A set of on-off flags packed into one number, one bit each.
- Bit packing: Storing several small values in the bytes of a larger value to save space.
- Buffer: A block of memory that holds a sequence of bytes, used to stage data.
- Byte: Eight bits, one unit of data. Files and buffers are sequences of bytes.
- Byte window: A slice of a file marked by offset and length, read as one unit.
- Cache: A store of computed results to avoid repeating work.
- Dependency injection: Passing dependencies into a component instead of letting it build them itself.
- Deserialize: Reading structured data back from bytes into memory. The reverse of serialize.
- Dirty tracking: Marking which cached pieces changed, so only those are rebuilt.
- Endianness: The order of bytes in a multi-byte number. See Little-endian in the file format section.
- Flag: One named on-off bit in a bitmask.
- Garbage collector: The Go runtime memory reclaimer. It finds unused memory and frees it in the background.
- Goroutine: A lightweight Go thread. Many can run on few OS threads.
- Hash: A number computed from data, used to spot changes or to key lookups.
- Heap: The memory pool for dynamic allocations. The Go runtime manages it automatically.
- Index: A number that names a position in a list, starting at zero.
- Interface: A Go contract of methods. Any type with those methods satisfies it. It lets subsystems swap implementations.
- Invariant: A condition that must stay true at all times, such as a format rule.
- Map: A lookup table from keys to values. In Go, the built-in `map` type.
- Mock: A fake implementation used in a test to isolate one unit.
- Mutex: A lock that lets one goroutine at a time enter a protected section.
- Offset: The byte distance from a start point to a target position. Vertex records and file records both use it to name where a field lives.
- Oracle: A reference implementation that produces the expected answer. C Ironwail is the oracle for this project.
- Parser: Code that reads a structured byte or text stream and builds data from it.
- Parity: Equality with the reference behavior or image. The project measures parity against C Ironwail.
- Pointer: A value that holds the address of another value. Go hides most pointer use behind slices and references.
- Queue: A structure that adds items at one end and removes them from the other, first in first out.
- Race condition: A bug where the order of concurrent operations changes the result. The race detector finds these.
- Recursion: A function that calls itself. BSP splits and portal flow use it.
- Reflection: The ability of Go to inspect types and values at runtime. It powered the old entity sync layer.
- Ring buffer: A fixed-size buffer that wraps around, overwriting the oldest data. Recent events stay available.
- Scalar: See Math and 3D coordinates.
- Scrap buffer: A reusable working buffer, allocated once and repurposed every frame.
- Serialize: Writing in-memory data into a byte stream.
- Slice: A Go view of a section of an array, with a start and a length.
- Stack: A structure that adds and removes items from the same end, last in first out.
- Stream: An ordered flow of bytes read or written little by little.
- Struct: A record of named fields, like a row in a table.
- Token: One unit of a command line or a file after splitting, such as a word or a quoted string.
- Tokenize: To split a text into tokens.

## The web build

- AudioWorklet: A browser feature that runs audio code on its own thread, used to feed the Oto audio stream to the Web Audio API.
- Canvas: The HTML element that holds the drawing surface in a browser. WebGPU renders into it.
- Deadzone: The small center range of a stick that is ignored, so the stick does not drift.
- Gyro: A rotation sensor inside a controller.
- JSON: A text format that stores data in named fields. Debug overlays send state to the browser as JSON.
- Keycode: The fixed code of a key, independent of layout. Quake keys use codes so configs stay portable.
- Pointer lock: A browser feature that captures the mouse, so a game can use relative movement.
- Rumble: The vibration motor inside a controller.
- Rune: A single Unicode character in Go. It handles international text better than a single byte.
- WASM: WebAssembly. A compact binary format that browsers run. The project compiles to it.