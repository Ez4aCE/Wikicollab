export type WsMessageType =
  | 'lock'
  | 'lock_renew'
  | 'unlock'
  | 'page_update'
  | 'lock_acquired'
  | 'lock_denied'
  | 'lock_released'
  | 'page_updated'
  | 'error';

export interface OutboundMessage {
  type: WsMessageType;
  block_id?: string;
  title?: string;
  content?: string;
  expected_version?: number;
}

export interface InboundMessage {
  type: WsMessageType;
  block_id?: string;
  user_id?: string;
  username?: string;
  page_id?: string;
  version?: number;
  content?: string;
  title?: string;
  message?: string;
}

export interface BlockData {
  id: string;
  content: string;
  lockedBy: string | null; // username if locked, null if free
  isOwnedByMe: boolean;    // true if the current user owns the lock
}
