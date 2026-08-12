import type { AttachmentPayload, AttachmentMeta } from "@app/types";
import type { PendingAttachment } from "../components/chat.types";

/** Function signature to convert a file to a Base64 encoded string. */
export type FileToBase64Fn = (file: File) => Promise<string>;

/** Function signature to optimize an image file before Base64 encoding. */
export type OptimizeImageFn = (file: File) => Promise<string>;

/** Function signature to convert PendingAttachment to AttachmentPayload for API. */
export type PendingToPayloadFn = (
  pa: PendingAttachment,
) => Promise<AttachmentPayload>;

/** Function signature to convert PendingAttachment to AttachmentMeta for local display. */
export type PendingToMetaFn = (pa: PendingAttachment) => AttachmentMeta;

/** Function signature to generate unique client-side IDs. */
export type NextIdFn = () => string;
