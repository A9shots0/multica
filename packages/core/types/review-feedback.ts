export interface ReviewMark {
  id: number;
  shot: string;
  x: number;
  y: number;
  w: number;
  h: number;
  note: string;
}

export interface CreateOwnerReviewFeedbackRequest {
  inbox_item_id?: string;
  source_attachment_id: string;
  marked_screenshot_attachment_id: string;
  marks: ReviewMark[];
  note: string;
}

export interface OwnerReviewFeedback extends CreateOwnerReviewFeedbackRequest {
  id: string;
  issue_id: string;
  source_filename: string;
  source_content_type: string;
  schema_version: 1;
  /** Current issue revision after the marked screenshot attachment was saved. */
  issue_revision: number;
  created_at: string;
}
