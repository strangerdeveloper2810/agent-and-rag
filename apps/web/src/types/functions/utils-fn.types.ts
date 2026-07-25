/** Function signature for file size formatting helper. */
export type FormatSizeFn = (bytes: number) => string;

/** Function signature for reading a file as Data URL. */
export type ReadAsDataURLFn = (file: File) => Promise<string>;

/** Function signature for theme switcher toggle handler. */
export type ToggleThemeFn = () => void;
