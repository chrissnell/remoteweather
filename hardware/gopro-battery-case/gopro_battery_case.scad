// GoPro battery + SD card case
// Cylindrical base holding 6 batteries and a bank of microSD cards,
// with a screw-on lid. ACME thread and diamond knurling via BOSL2.
//
// Reference design holds camera + batteries + SDs; this variant drops the
// camera cavity and packs batteries + cards only.

include <BOSL2/std.scad>
include <BOSL2/threading.scad>

/* [Part] */
// base = threaded body, lid = knurled cap, assembly = both (open), closed = cross-section
part = "assembly";   // [base, lid, assembly, closed]

/* [Body] */
body_d      = 104;   // knurled outer diameter of the body
body_h      = 40;    // total body height
neck_major  = 96;    // ACME thread major (crest) diameter of the top neck
neck_h      = 10;    // threaded neck height / engagement length (short = less travel)
floor_h     = 8;     // solid floor under the deepest pocket (strength)

/* [Lid] */
lid_h          = 32; // total lid height (= neck_h + lid_relief + lid_top)
lid_top        = 4;  // solid thickness of the lid roof
lid_relief     = 18; // clear space above the body top for proud batteries/cards (>=16)

/* [Thread] */
thread_pitch = 2.5;  // ACME pitch (finer thread)
slop         = 0.15; // printer clearance added to the internal (lid) thread

/* [Batteries] */
// Slot dimensions per spec (34 x 13.5 mm, 29 mm deep).
batt_slot_w    = 34;  // slot size along Y (battery width)
batt_slot_t    = 13.5;// slot size along X (battery thickness)
batt_slot_depth= 29;  // pocket depth
batt_wall_x    = 3;   // wall between columns
batt_wall_y    = 4;   // wall between the two rows

/* [microSD] */
// Slot dimensions per spec (12.5 x 1 mm, 10 mm deep). Cards are pulled away from
// the batteries and spread into the free space so each one can be pinched:
//   - inner column of 4 per side, moved outboard from the battery block
//   - 2 outboard slots per side, pushed toward the edge
//   - 2 slots relocated above the batteries, 2 below (one from each side)
sd_slot_w      = 12.5; // slot width (card width)
sd_slot_t      = 1;    // slot thickness (card thickness)
sd_slot_depth  = 10;   // pocket depth
sd_inner_x     = 30;   // X of the inner column of 4 (off the batteries)
sd_inner_rp    = 14.5; // row pitch of the inner column
sd_outer_x     = 38;   // X of the 2 outboard slots (toward the edge)
sd_topbot_x    = 8;    // X offset of the relocated top/bottom slots
sd_topbot_y    = 39.5; // Y of the relocated slots (above / below the batteries)

/* [Knurling] */
knurl      = true;      // diamond knurl on the outer walls (disable for fast preview)
knurl_size = 4;         // diamond cell size

/* [Edges] */
edge_chamfer = 3;       // 45 deg smooth (un-knurled) chamfer on base bottom & lid top

/* [Logo] */
logo_enable = true;         // inlay the GoPro logo flush into the lid top
logo_file   = "GoPro_logo_light.svg";
logo_svg_w  = 77.1;         // logo SVG viewBox width (do not change)
logo_width  = 60;           // logo size across the lid top (mm)
logo_depth  = 1.2;          // inlay depth; top sits flush with the lid (mm)

/* [Quality] */
$fa = 2;
$fs = 0.6;

// ---------------------------------------------------------------------------

corner_round = 2;       // fillet radius of pocket corners
eps = 0.05;

// Column X positions for the 3 battery columns.
function batt_cols() = let(p = batt_slot_t + batt_wall_x) [-p, 0, p];
// Row Y positions for the 2 battery rows.
function batt_rows() = let(p = (batt_slot_w + batt_wall_y)/2) [-p, p];

module rounded_slot(sx, sy, depth) {
    // Vertical pocket, open at the top, rounded vertical corners.
    r = min(corner_round, min(sx, sy)/2 - 0.01);
    translate([0,0,-depth])
        linear_extrude(depth + eps)
            offset(r=r) offset(delta=-r)
                square([sx, sy], center=true);
}

module battery_pockets() {
    for (x = batt_cols(), y = batt_rows())
        translate([x, y, 0]) rounded_slot(batt_slot_t, batt_slot_w, batt_slot_depth);
}

// Centred offsets for n items at the given pitch.
function centred(n, pitch) = [for (i = [0:n-1]) (i - (n-1)/2) * pitch];

module sd_v() rounded_slot(sd_slot_t, sd_slot_w, sd_slot_depth);  // card width along Y
module sd_h() rounded_slot(sd_slot_w, sd_slot_t, sd_slot_depth);  // card width along X

module sd_pockets() {
    rows = centred(4, sd_inner_rp);      // 4 inner rows
    mid  = sd_inner_rp / 2;              // middle-row Y for the outboard pair
    for (side = [-1, 1]) {
        // inner column of 4, off the batteries
        for (y = rows) translate([side*sd_inner_x, y, 0]) sd_v();
        // 2 outboard slots at the middle rows, toward the edge
        for (y = [-mid, mid]) translate([side*sd_outer_x, y, 0]) sd_v();
        // 1 slot above and 1 below the batteries (one from each side)
        translate([side*sd_topbot_x,  sd_topbot_y, 0]) sd_h();
        translate([side*sd_topbot_x, -sd_topbot_y, 0]) sd_h();
    }
}

sd_count = 2 * (4 + 2 + 2);   // total microSD slots

// Knurled cylindrical wall with optional smooth 45 deg chamfers at each end.
// The chamfers are plain (un-knurled) conical faces.
module knurled_wall(d, h, cham_b=0, cham_t=0) {
    band_h = h - cham_b - cham_t;
    up(cham_b) {
        if (knurl)
            cyl(d=d, h=band_h, texture="diamonds", tex_size=[knurl_size, knurl_size],
                style="concave", anchor=BOTTOM);
        else
            cyl(d=d, h=band_h, anchor=BOTTOM);
    }
    if (cham_b > 0) cyl(d1=d - 2*cham_b, d2=d, h=cham_b, anchor=BOTTOM);
    if (cham_t > 0) up(h - cham_t) cyl(d1=d, d2=d - 2*cham_t, h=cham_t, anchor=BOTTOM);
}

module base() {
    lower_h = body_h - neck_h;
    difference() {
        union() {
            // Lower knurled barrel; smooth chamfer on the exposed bottom edge.
            knurled_wall(body_d, lower_h, cham_b=edge_chamfer);
            // Threaded neck on top, reduced so the lid sits flush with the body.
            up(lower_h) acme_threaded_rod(d=neck_major, l=neck_h, pitch=thread_pitch,
                                          bevel1=false, anchor=BOTTOM);
        }
        // Pockets cut from the top face down.
        up(body_h) {
            battery_pockets();
            sd_pockets();
        }
    }
}

// GoPro logo as a 2D shape, scaled and centred on the lid top.
module logo_2d() {
    s = logo_width / logo_svg_w;
    scale([s, s]) import(logo_file, center=true);
}

// The logo inlay solid: fills the top logo_depth of the roof, top face flush
// with the lid top (z = lid_h). Print this in the second filament.
module logo_inlay() {
    up(lid_h - logo_depth) linear_extrude(logo_depth) logo_2d();
}

module lid() {
    thread_h = neck_h;                      // internal thread engagement
    cavity_h = neck_h + lid_relief;         // skirt depth: engagement + relief
    relief_d = neck_major + 1;              // clear bore above the thread
    // Model opening-down: opening at z=0, roof at z=lid_h.
    difference() {
        knurled_wall(body_d, lid_h, cham_t=edge_chamfer);  // knurled cup, chamfered top edge
        // Relief bore above the thread region.
        up(thread_h - eps)
            cyl(d=relief_d, h=cavity_h - thread_h + eps, anchor=BOTTOM);
        // Internal ACME thread at the opening.
        acme_threaded_rod(d=neck_major, l=thread_h + eps, pitch=thread_pitch,
                          internal=true, bevel2=false, $slop=slop, anchor=BOTTOM);
        // Logo pocket in the top, flush with the surface.
        if (logo_enable)
            up(lid_h - logo_depth) linear_extrude(logo_depth + eps) logo_2d();
    }
    // Sanity: roof thickness must match.
    assert(lid_h - cavity_h >= lid_top - eps, "lid_h too short for cavity + roof");
    assert(lid_relief >= 16, "lid_relief must be >= 16 mm");
    assert(!logo_enable || logo_depth < lid_top, "logo_depth must be < lid_top");
}

module assembly(closed=false) {
    base();
    // Lid (modelled opening-down) dropped onto the neck shoulder.
    shoulder_z = body_h - neck_h;
    color("SlateBlue")
        translate([0,0, shoulder_z]) lid();
}

if (part == "base") base();
else if (part == "lid") lid();
else if (part == "logo") logo_inlay();               // second-filament inlay
else if (part == "cap_logo") {                        // preview: lid + logo
    lid();
    color("#00AEEF") logo_inlay();
}
else if (part == "assembly") assembly();
else if (part == "closed")
    difference() { assembly(); translate([0,-200,-50]) cube([200,400,400]); }
else if (part == "cutaway")
    difference() { assembly(); translate([-1,-1,-50]) cube([200,200,400]); }
else if (part == "slab")   // thin cross-section through the centre battery column
    intersection() { assembly(); translate([-1,-200,-50]) cube([2,400,400]); }
