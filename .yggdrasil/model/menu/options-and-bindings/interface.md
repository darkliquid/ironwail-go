# Interface

## Main consumers

- `menu/state-machine`, which dispatches here for `MenuOptions`, `MenuControls`, `MenuVideo`, and `MenuAudio`.

## Main surface

- `optionsKey`, `controlsKey`, `videoKey`, `audioKey`
- setting-adjustment helpers (`adjustControlSetting`, `adjustVideoSetting`, `adjustAudioSetting`)
- binding helpers (`setControlBinding`, `clearControlBinding`, `keysForBinding`, `controlBindingLabel`)
- draw helpers for options/controls/video/audio screens

## Contracts

- Controls rebinding enters a capture mode where the next key press becomes the new binding unless cancelled.
- Most options mutate cvars immediately rather than waiting for an explicit apply step.
- Video and control rows use hard-coded menu indices and row-position helpers to keep cursor behavior aligned with the on-screen layout.
