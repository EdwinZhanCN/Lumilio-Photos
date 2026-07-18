type OffsetPagination = {
  limit?: number;
  offset?: number;
};

export const withBodyPaginationOffset = <T extends { pagination?: OffsetPagination }>(
  request: T,
  offset: number,
): T => ({
  ...request,
  pagination: {
    ...request.pagination,
    offset,
  },
});
