# Interface

## Main consumers

- renderer/runtime code that supplies a `DrawContext`
- runtime/UI code that decides whether to draw the full console or only notify lines
- layout code that needs notify-line counts

## Main surface

- `Draw` and `(*Console).Draw`
- `DrawContext`
- notify-line count helpers
- internal helpers for prompt clipping, cursor blink, and notify alpha

## Contracts

- Drawing is backend-neutral and must work through the small `DrawContext` surface.
- Draw paths snapshot console state under read locks, then render outside the lock.
- Notify visibility depends on `con_notify*` cvars and timestamp state owned by the console core.
