import React, { useState, useContext, useEffect, useRef } from "react";
import PropTypes from "prop-types";
import { parseISO, formatRelative } from "date-fns";
import {
  More,
  Add,
  UserAdd,
  Download,
  SubtractCircle,
  FormPreviousLink,
  FormNextLink,
} from "grommet-icons";
import {
  TableHeader,
  TableBody,
  TableRow,
  TableCell,
  Box,
  Button,
  Heading,
  Select,
} from "grommet";

import history from "../history";
import { useApi, useInterval } from "../hooks";
import { mainInstance as axios } from "../axios";
import { StyledTable, PlaceholderTable, Modal, SecondaryButton } from "../ui";
import { NotificationsContext } from "../Notifications/context";
import CreateSubscriber from "./Create";
import DeleteSubscriber from "./Delete";
import EditSubscriber from "./Edit";

export const Row = ({ subscriber, actions }) => {
  const ca = parseISO(subscriber.created_at);
  const ua = parseISO(subscriber.updated_at);
  return (
    <TableRow>
      <TableCell scope="row" size="medium">
        <strong>{subscriber.email}</strong>
      </TableCell>
      <TableCell scope="row" size="medium">
        {formatRelative(ca, new Date())}
      </TableCell>
      <TableCell scope="row" size="medium">
        {formatRelative(ua, new Date())}
      </TableCell>
      <TableCell scope="row" size="xsmall" align="end">
        {actions}
      </TableCell>
    </TableRow>
  );
};

Row.propTypes = {
  subscriber: PropTypes.shape({
    email: PropTypes.string,
    id: PropTypes.number,
    created_at: PropTypes.string,
    updated_at: PropTypes.string,
  }),
  actions: PropTypes.element,
};

export const Header = () => (
  <TableHeader>
    <TableRow>
      <TableCell scope="col" border="bottom" size="medium">
        <strong>Email</strong>
      </TableCell>
      <TableCell scope="col" border="bottom" size="medium">
        <strong>Created At</strong>
      </TableCell>
      <TableCell scope="col" border="bottom" size="small">
        <strong>Updated At</strong>
      </TableCell>
      <TableCell align="end" scope="col" border="bottom" size="small">
        <strong>Action</strong>
      </TableCell>
    </TableRow>
  </TableHeader>
);

export const SubscriberTable = React.memo(({ list, actions }) => (
  <StyledTable>
    <Header />
    <TableBody>
      {list.map((s) => (
        <Row subscriber={s} key={s.id} actions={actions(s)} />
      ))}
    </TableBody>
  </StyledTable>
));

SubscriberTable.displayName = "SubscriberTable";
SubscriberTable.propTypes = {
  list: PropTypes.array,
  actions: PropTypes.func,
};

// eslint-disable-next-line react/display-name
const rowActions = (setShowEdit, setShowDelete) => (subscriber) => {
  return (
    <Select
      alignSelf="center"
      plain
      icon={<More />}
      options={["Edit", "Delete"]}
      onChange={({ option }) => {
        (function () {
          switch (option) {
            case "Edit":
              setShowEdit({
                show: true,
                id: subscriber.id,
              });
              break;
            case "Delete":
              setShowDelete({
                show: true,
                email: subscriber.email,
                id: subscriber.id,
              });
              break;
            default:
              return null;
          }
        })();
      } } />
  );
};

const ExportSubscribers = () => {
  const linkEl = useRef(null);
  const { createNotification } = useContext(NotificationsContext);
  const [notification, setNotification] = useState();
  const [filename, setFilename] = useState("");
  const [retries, setRetries] = useState(-1);
  const [state, callApi] = useApi({
    url: `/api/subscribers/export`,
  }, null, true);

  useInterval(
    async () => {
      await callApi({ url: `/api/subscribers/export/download?filename=${filename}` });
      setRetries(retries - 1);
    },
    retries > 0 ? 1000 : null
  );

  useEffect(() => {
    if (notification) {
      createNotification(notification.message, notification.status);
    }
  }, [notification]);

  useEffect(() => {
    if (!state.isLoading && state.isError && state.data) {
      if (state.data.status === "failed" && retries > 0 && retries < 50) {
        setRetries(-1);
        setNotification({message: state.data.message, status: "status-error"});
      }
    }
  
    if (!state.isLoading && !state.isError && state.data) {
      if (retries > 0) {
        setRetries(-1);
        linkEl.current.click()
      }
    }
  }, [state]);
  

  return (
    <>
    <SecondaryButton
      disabled={retries > 0 || state.isLoading}
      onClick={
        async () => {
          try {
            const res = await axios.post(`/api/subscribers/export`);
            setFilename(res.data.file_name);
            setRetries(50);
          } catch(e) {
            console.error("Unable to generate report", e);
          }
        }
        } icon={<Download size="20px" />} label="Export"
    />
    {
      !state.isLoading && !state.isError && state.data && 
      <a ref={linkEl} href={state.data.url}></a>
    }
    </>
  )
}

const ActionButtons = () => (
  <>
    <SecondaryButton
      margin={{ right: "small" }}
      icon={<UserAdd size="20px" />}
      label="Import from file"
      onClick={() => history.push("/dashboard/subscribers/import")}
    />
    <SecondaryButton
      margin={{ right: "small" }}
      icon={<SubtractCircle size="20px" />}
      label="Delete from file"
      onClick={() => history.push("/dashboard/subscribers/bulk-delete")}
    />
    <ExportSubscribers />
  </>
);

const List = () => {
  const [showDelete, setShowDelete] = useState({
    show: false,
    email: "",
    id: "",
  });
  const [showEdit, setShowEdit] = useState({ show: false, id: "" });
  const [showCreate, openCreateModal] = useState(false);
  const hideDeleteModal = () =>
    setShowDelete({ show: false, email: "", id: "" });
  const hideEditModal = () => setShowEdit({ show: false, id: "" });

  const [state, callApi] = useApi(
    {
      url: "/api/subscribers",
    },
    {
      collection: [],
      init: true,
    }
  );

  let table = null;
  if (state.isLoading) {
    table = <PlaceholderTable header={Header} numCols={3} numRows={10} />;
  } else if (state.data.collection.length > 0) {
    table = (
      <SubscriberTable
        isLoading={state.isLoading}
        list={state.data.collection}
        actions={rowActions(setShowEdit, setShowDelete)}
      />
    );
  }

  return (
    <>
      {showDelete.show && (
        <Modal
          title={`Delete subscriber ${showDelete.email} ?`}
          hideModal={hideDeleteModal}
          form={
            <DeleteSubscriber
              id={showDelete.id}
              callApi={callApi}
              hideModal={hideDeleteModal}
            />
          }
        />
      )}
      {showCreate && (
        <Modal
          title={`Create subscriber`}
          hideModal={() => openCreateModal(false)}
          form={
            <CreateSubscriber
              callApi={callApi}
              hideModal={() => openCreateModal(false)}
            />
          }
        />
      )}
      {showEdit.show && (
        <Modal
          title={`Edit subscriber`}
          hideModal={hideEditModal}
          form={
            <EditSubscriber
              id={showEdit.id}
              callApi={callApi}
              hideModal={hideEditModal}
            />
          }
        />
      )}
      <Box gridArea="nav" direction="row">
        <Box alignSelf="center" margin={{ right: "small" }}>
          <Heading level="2">Subscribers</Heading>
        </Box>
        <Box alignSelf="center">
          <Button
            primary
            color="status-ok"
            label="Create new"
            icon={<Add />}
            reverse
            onClick={() => openCreateModal(true)}
          />
        </Box>
        <Box margin={{ left: "auto" }} alignSelf="center" direction="row">
          <ActionButtons />
        </Box>
      </Box>
      <Box gridArea="main">
        <Box animation="fadeIn">
          {table}

          {!state.isLoading && state.data.collection.length === 0 ? (
            <Box align="center" margin={{ top: "large" }}>
              <Heading level="2">Create your first subscriber.</Heading>
            </Box>
          ) : null}
        </Box>
        {!state.isLoading && state.data.collection.length > 0 ? (
          <Box direction="row" alignSelf="end" margin={{ top: "medium" }}>
            <Box margin={{ right: "small" }}>
              <Button
                icon={<FormPreviousLink />}
                label="Previous"
                disabled={state.data.links.previous === null}
                onClick={() => {
                  callApi({
                    url: state.data.links.previous,
                  });
                }}
              />
            </Box>
            <Box>
              <Button
                icon={<FormNextLink />}
                reverse
                label="Next"
                disabled={state.data.links.next === null}
                onClick={() => {
                  callApi({
                    url: state.data.links.next,
                  });
                }}
              />
            </Box>
          </Box>
        ) : null}
      </Box>
    </>
  );
};

export default List;
