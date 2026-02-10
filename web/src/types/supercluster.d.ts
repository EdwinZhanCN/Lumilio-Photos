declare module "supercluster" {
  export type BBox = [number, number, number, number];

  export type PointGeometry = {
    type: "Point";
    coordinates: [number, number];
  };

  export interface PointFeature<P = Record<string, unknown>> {
    type: "Feature";
    id?: number | string;
    geometry: PointGeometry;
    properties: P;
  }

  export interface ClusterProperties {
    cluster: true;
    cluster_id: number;
    point_count: number;
    point_count_abbreviated: number | string;
  }

  export type ClusterFeature<C = Record<string, unknown>> = PointFeature<
    C & ClusterProperties
  >;

  export interface SuperclusterOptions<
    P = Record<string, unknown>,
    C = Record<string, unknown>,
  > {
    minZoom?: number;
    maxZoom?: number;
    minPoints?: number;
    radius?: number;
    extent?: number;
    nodeSize?: number;
    log?: boolean;
    generateId?: boolean;
    map?: (properties: P) => C;
    reduce?: (accumulated: C, properties: C) => void;
  }

  export interface TileFeature<P = Record<string, unknown>> {
    type: 1;
    geometry: [number, number][];
    tags: ClusterProperties | P;
  }

  export interface Tile<P = Record<string, unknown>> {
    features: Array<TileFeature<P>>;
  }

  export default class Supercluster<
    P = Record<string, unknown>,
    C = Record<string, unknown>,
  > {
    constructor(options?: SuperclusterOptions<P, C>);
    load(points: Array<PointFeature<P>>): Supercluster<P, C>;
    getClusters(
      bbox: BBox,
      zoom: number,
    ): Array<PointFeature<P> | ClusterFeature<C>>;
    getChildren(clusterId: number): Array<PointFeature<P> | ClusterFeature<C>>;
    getLeaves(
      clusterId: number,
      limit?: number,
      offset?: number,
    ): Array<PointFeature<P>>;
    getTile(z: number, x: number, y: number): Tile<P> | null;
    getClusterExpansionZoom(clusterId: number): number;
  }
}
