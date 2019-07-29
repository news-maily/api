import styled from "styled-components";
import { Table } from "grommet";

const StyledTable = styled(Table)`
  background: white;
  color: #4f566b;
  width: 100%;
  border-radius: 4px;
  box-shadow: 0 7px 14px 0 rgba(60, 66, 87, 0.1),
    0 3px 6px 0 rgba(0, 0, 0, 0.07);

  ${this} tr:hover, focus {
    background: fafafa;
  }
  ${this} tr:focus {
    background: fafafa;
  }

  ${this} tr {
    border-bottom: 1px solid #f0f0f0;
  }

  ${this} thead {
    color: #6650aa;
    text-transform: uppercase;
  }

  ${this} thead th div {
    border-bottom: none;
  }

  ${this} tbody {
    color: #4f566b;
  }

  ${this} tbody tr:hover {
    background: #fafafa;
  }
`;

export default StyledTable;
