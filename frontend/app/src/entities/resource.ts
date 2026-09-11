/** 领域实体:前端展示模型(与 openapi 生成的 schema 类型对齐,新增字段需先动契约)。 */

export interface DataCenterNode {
  id: string;
  code: string;
  name: string;
  version: number;
  rooms?: RoomNode[];
}

export interface RoomNode {
  id: string;
  code: string;
  name: string;
  version: number;
  racks?: RackNode[];
}

export interface RackNode {
  id: string;
  code: string;
  name: string;
  uHeight?: number;
  version: number;
}

export interface DeviceRow {
  id: string;
  code: string;
  name: string;
  heightU: number;
  lifecycleStatus: string;
  version: number;
  currentPosition?: {
    rackId?: string;
    rackCode?: string;
    rackName?: string;
    startU?: number;
    endU?: number;
    orientation?: string;
  };
}
