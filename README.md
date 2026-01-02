# 🛩️ Dronnayak

**An open exploration into drone telemetry, connectivity, and autonomous drone systems.**

Dronnayak is an experimental project focused on understanding and building the *core infrastructure* that makes drones connected, observable, and eventually autonomous.

This repository contains the early work around a **drone telemetry device and platform** built using:

* Pixhawk / ArduPilot / PX4
* MAVLink
* Raspberry Pi (companion computer)
* Backend systems (Go-based services)
* Simple web dashboards

> This is **not a startup pitch**.
> This is a learning-driven, open systems project.

---

##  Motivation

Modern drones are powerful, but they are often **isolated systems**:

* Telemetry is limited to short-range radios
* Observability disappears once the drone is out of range
* Fleet-level visibility is hard
* Remote operations and autonomy require fragile setups

As a backend engineer entering the drone space, I wanted to understand:

* How drones actually *communicate*
* How MAVLink works at a protocol level
* How telemetry can be streamed reliably over unreliable networks
* How edge devices (companion computers) can bridge drones to the cloud

Dronnayak is my attempt to **learn these fundamentals deeply by building them**.

---

##  What Dronnayak Is (Right Now)

At its current stage, Dronnayak is a **proof-of-concept** that demonstrates:

* Reading MAVLink telemetry from a Pixhawk via UART
* Running a lightweight companion computer (Raspberry Pi Zero)
* Parsing basic MAVLink messages (heartbeat, GPS, battery, attitude)
* Streaming telemetry to a backend service
* Viewing live telemetry in a simple web dashboard

This is intentionally minimal and imperfect.
The goal is **understanding, not completeness**.

---

##  What This Project Is NOT

To set clear expectations:

*  Not production-ready
*  Not a commercial product (yet)
*  Not optimized for scale
*  Not a full Ground Control Station
*  Not a promise of future features

This repo exists to **explore, learn, and discuss**.

---

##  High-Level Architecture

```
[ Drone / Pixhawk ]
        |
        |  MAVLink (UART)
        |
[ Companion Computer (Raspberry Pi) ]
        |
        |  WebSocket / HTTP
        |
[ Backend Service ]
        |
        |  Browser
        |
[ Telemetry Dashboard ]
```

Key ideas being explored:

* Edge → cloud telemetry pipelines
* Handling intermittent connectivity
* Simple, inspectable data flows
* Clear separation between device, transport, and visualization

---


##  Current Focus

Right now, the focus areas are:

* MAVLink fundamentals
* Telemetry reliability
* Reconnection & buffering strategies
* Clean data models
* Simple observability (logs > magic)

Advanced topics like autonomy, AI, swarms, or BVLOS are **deliberately out of scope for now**.

---

## 🤝 Collaboration & Feedback

This project is open because learning is faster with other minds involved.

If you are:

* Working with MAVLink / PX4 / ArduPilot
* Interested in drone systems & autonomy
* Experienced with embedded systems or distributed systems
* Curious and thoughtful

…feedback, reviews, and discussions are welcome.

Ways to contribute:

* Open an issue with suggestions or questions
* Comment on architecture decisions
* Share similar experiences or pitfalls
* Improve documentation or clarity

No pressure to “contribute code”.

---

## 📜 License

This project is released under the **MIT License**.

You are free to:

* Use
* Modify
* Fork
* Learn from

Attribution is appreciated.
Liability is not accepted.

---

## 🧭 Project Philosophy

A few guiding principles:

* Learning > Speed
* Clarity > Cleverness
* Calm progress > Hustle
* Systems thinking > Feature chasing

This project will move **slowly, intentionally, and openly**.

---

## 👤 About the Author

I’m Kunal — a backend engineer exploring the drone domain through hands-on system building.

I’m particularly interested in:

* Drone connectivity
* Telemetry pipelines
* Edge + cloud systems
* Autonomy infrastructure

This project reflects my learning journey in public.

---

## 📌 Status

🟡 **Early Proof of Concept**
Things may break. APIs may change. Assumptions may be wrong.

That’s expected.

---

If you read this far — thank you.
And if something here resonates, feel free to reach out or open a discussion.