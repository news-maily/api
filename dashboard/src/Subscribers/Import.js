import React, { useState, useEffect, useContext, useReducer } from "react";
import { Box, Heading, Markdown, Select, Text } from "grommet";
import Uppy from "@uppy/core";
import AwsS3 from "@uppy/aws-s3";
import { DragDrop, StatusBar } from "@uppy/react";
import qs from "qs";

import "@uppy/core/dist/style.css";
import "@uppy/drag-drop/dist/style.css";
import "@uppy/status-bar/dist/style.css";

import { mainInstance as axios } from "../axios";
import { useApi } from "../hooks";
import { NotificationsContext } from "../Notifications/context";

const Content = `
CSV format:

- Columns should be separated by comma
- Number and order of columns should match the example below
- Each column after the **Name** will be included in the subscriber's **metadata** (you can use these fields in your templates)

Example:

**Email** | **Name** | **metadata1** | **metadata2** | ...
--- | --- | --- | --- |
john@example.com | John Doe | foo | bar | ...
jane@example.com | Jane Doe | fizz | buzz | ...
`;

const ImportSubscribers = () => {
  const { createNotification } = useContext(NotificationsContext);
  const [selected, setSelected] = useState([]);
  const [segments, callApi] = useApi(
    {
      url: `/api/segments?per_page=40`,
    },
    {
      collection: [],
      links: {
        next: null,
      },
    }
  );

  const reducer = (state, action) => {
    switch (action.type) {
      case "append":
        return [...state, ...action.payload];
      default:
        throw new Error("invalid action type.");
    }
  };

  const [options, dispatch] = useReducer(reducer, []);

  useEffect(() => {
    if (segments.isError || segments.isLoading) {
      return;
    }

    if (segments && segments.data) {
      dispatch({ type: "append", payload: segments.data.collection });
    }
  }, [segments]);

  const onMore = () => {
    if (segments.isError || segments.isLoading) {
      return;
    }

    let url = "";
    if (segments && segments.data && segments.data.links) {
      url = segments.data.links.next;
    }

    if (!url) {
      return;
    }

    callApi({
      url: url,
    });
  };

  const onChange = ({ value: nextSelected }) => {
    setSelected(nextSelected);
  };

  const uppy = Uppy({
    restrictions: {
      maxNumberOfFiles: 1,
      allowedFileTypes: ["text/csv"],
    },
  });
  uppy.use(AwsS3, {
    async getUploadParameters(file) {
      try {
        const res = await axios.post(
          "/api/s3/sign",
          qs.stringify({
            filename: file.name,
            contentType: file.type,
            action: "import",
          })
        );

        return res.data;
      } catch (error) {
        let msg = "Unable to upload file. Please try again.";
        if (error.response) {
          msg = error.response.data.message;
        }

        createNotification(msg, "status-error");
      }
    },
  });

  uppy.on("upload-success", async (file) => {
    try {
      const res = await axios.post(
        "/api/subscribers/import",
        qs.stringify(
          {
            filename: file.name,
            segments: selected.map((s) => s.id),
          },
          { arrayFormat: "brackets" }
        )
      );

      createNotification(res.data.message, "status-ok");
    } catch (error) {
      let msg = "Unable to import subscribers. Please try again.";
      if (error.response) {
        msg = error.response.data.message;
      }

      createNotification(msg, "status-error");
    }

    uppy.reset();
  });

  useEffect(() => {
    return () => {
      uppy.close();
    };
  }, [uppy]);

  return (
    <Box direction="column" margin="medium" animation="fadeIn">
      <Box pad={{ left: "medium" }} margin={{ bottom: "small" }}>
        <Heading level="2">Import from a CSV file</Heading>
      </Box>
      <Box round background="white" pad="medium" width="50%" alignSelf="start">
        <Markdown>{Content}</Markdown>
        <Box margin={{ top: "medium" }}>
          <Text margin={{ bottom: "small" }}>Add to segments (optional)</Text>
          <Select
            multiple
            closeOnChange={false}
            placeholder="select segments..."
            value={selected}
            labelKey="name"
            valueKey="id"
            options={options}
            dropHeight="medium"
            onMore={onMore}
            onChange={onChange}
          />
        </Box>
        <Box margin={{ top: "large" }}>
          <DragDrop
            width="100%"
            height="100%"
            note="Only CSV files are allowed"
            uppy={uppy}
          />
          <StatusBar hideAfterFinish={false} showProgressDetails uppy={uppy} />
        </Box>
      </Box>
    </Box>
  );
};

export default ImportSubscribers;
