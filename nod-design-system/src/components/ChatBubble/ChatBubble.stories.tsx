import React from "react";
import { ChatBubble } from "./ChatBubble";

export default { title: "Components/ChatBubble", component: ChatBubble };

/** Message sent by the current user — right-aligned, darker background */
export const SentMessage = () => (
  <ChatBubble content="Hey, are you on the LAN?" timestamp="2:34 PM" isSent={true} />
);

/** Message received from a peer — left-aligned, elevated surface */
export const ReceivedMessage = () => (
  <ChatBubble
    content="Yeah, discovered you via UDP. Connection is peer-to-peer."
    timestamp="2:35 PM"
    isSent={false}
    senderName="rogue"
  />
);

/** Long message wrapping */
export const LongMessage = () => (
  <ChatBubble
    content="This is a longer message to demonstrate how text wraps within the bubble. The max-width is set to 65ch for optimal readability following typographic best practices."
    timestamp="2:36 PM"
    isSent={false}
    senderName="alice"
  />
);
